package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leighmacdonald/steamid/v4/steamid"
	"github.com/pkg/errors"
)

type Attributes string

const (
	cheater   Attributes = "cheater"
	racist    Attributes = "racist"
	sus       Attributes = "suspicious"
	exploiter Attributes = "exploiter"
)

const (
	masterList  = "https://raw.githubusercontent.com/PazerOP/tf2_bot_detector/master/tf2_bot_detector/cfg/playerlist.official.json"
	schemaURL   = "https://raw.githubusercontent.com/PazerOP/tf2_bot_detector/master/schemas/v3/playerlist.schema.json"
	title       = "@trusted Ban List"
	description = "Curated list of steamid's created by the @trusted people in the official discord server"
	updateURL   = "https://trusted.roto.lol/v1/steamids"
)

type ListSource struct {
	Authors     []string `json:"authors"`
	Description string   `json:"description"`
	Title       string   `json:"title"`
	UpdateURL   string   `json:"update_url"`
}

type masterListResp struct {
	ListSource ListSource `json:"file_info"`
	Schema     string     `json:"$schema"`
	Players    []Player   `json:"players"`
	Version    int        `json:"version"`
}

type AddSteamIDReq struct {
	Attributes []Attributes
	SteamID    string
	Username   string
}

type LastSeen struct {
	PlayerName string `json:"player_name"`
	Time       int64  `json:"time"`
}

type Player struct {
	SteamID    steamid.SteamID `json:"steamid"`
	Attributes []Attributes    `json:"attributes"`
	LastSeen   LastSeen        `json:"last_seen,omitempty"`
}

type ErrResp struct {
	Error string `json:"error"`
}

type SucResp struct {
	Message string `json:"message"`
}

type App struct {
	db    *sql.DB
	ids   map[steamid.SteamID]Player
	idsMu *sync.RWMutex
	ctx   context.Context
}

func NewApp(ctx context.Context, dbPath string) (*App, error) {
	db, err := openDB(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	players := make(map[steamid.SteamID]Player)
	if err := loadPlayers(ctx, db, players); err != nil {
		return nil, err
	}

	return &App{
		db:    db,
		ids:   players,
		idsMu: &sync.RWMutex{},
		ctx:   context.Background(),
	}, nil
}

func newSteamIDResp(players []Player) masterListResp {
	return masterListResp{
		ListSource: ListSource{
			Authors:     []string{"pazer"},
			Description: description,
			Title:       title,
			UpdateURL:   updateURL,
		},
		Schema:  schemaURL,
		Players: players,
	}
}

func (a *App) handleGetSteamIDS(c *gin.Context) {
	var players []Player
	a.idsMu.RLock()
	for _, v := range a.ids {
		players = append(players, v)
	}
	a.idsMu.RUnlock()
	c.JSON(200, newSteamIDResp(players))
}

func (a *App) handleAddSteamID(c *gin.Context) {
	var req AddSteamIDReq
	if err := c.BindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrResp{Error: "Invalid request format"})
		return
	}
	steamID := steamid.New(req.SteamID)
	if !steamID.Valid() {
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrResp{
			Error: fmt.Sprintf("Invalid steam id: %s", req.SteamID),
		})
		return
	}
	a.idsMu.Lock()
	a.ids[steamID] = Player{
		Attributes: req.Attributes,
		SteamID:    steamID,
	}
	a.idsMu.Unlock()
	c.JSON(200, SucResp{Message: fmt.Sprintf("Added successfully: %s", req.SteamID)})
}

func DownloadMasterList() ([]Player, error) {
	resp, err := http.Get(masterList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get master list: %s", err)
	}
	if resp.StatusCode != 200 {
		return nil, errors.Wrapf(err, "Invalid status code from gh: %d", resp.StatusCode)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("Failed to close response body", slog.String("error", err.Error()))
		}
	}()

	var listResp masterListResp
	if errDecode := json.NewDecoder(resp.Body).Decode(&listResp); errDecode != nil {
		return nil, errors.Wrapf(errDecode, "Failed to decide master list: %s", errDecode)
	}
	var p []Player
	for _, mlP := range listResp.Players {
		newPlayer := Player{
			Attributes: mlP.Attributes,
			LastSeen:   mlP.LastSeen,
			SteamID:    mlP.SteamID,
		}
		if newPlayer.SteamID.Valid() {
			p = append(p, newPlayer)
		}
	}
	return p, nil
}

func (a *App) LoadMasterIDS(ml []Player) int {
	added := 0
	for _, p := range ml {
		if err := addPlayer(a.ctx, a.db, p); err != nil {
			if err.Error() == "UNIQUE constraint failed: player.steamid" {
				continue
			}
			slog.Error(err.Error())
		}
		added++
	}
	a.idsMu.Lock()
	for _, p := range ml {
		a.ids[p.SteamID] = p
	}
	a.idsMu.Unlock()
	slog.Info("Downloaded master list success")
	return added
}

type HTTPOpts struct {
	ListenAddr     string
	UseTLS         bool
	Handler        http.Handler
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxHeaderBytes int
	TLSConfig      *tls.Config
}

func DefaultHTTPOpts() *HTTPOpts {
	return &HTTPOpts{
		ListenAddr:     ":8899",
		UseTLS:         false,
		Handler:        nil,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}

func NewRouter(a *App) *gin.Engine {
	r := gin.New()
	r.GET("/v1/steamids", a.handleGetSteamIDS)
	return r
}

func NewHTTPServer(opts *HTTPOpts) *http.Server {
	return &http.Server{
		Addr:           opts.ListenAddr,
		Handler:        opts.Handler,
		ReadTimeout:    opts.ReadTimeout,
		WriteTimeout:   opts.WriteTimeout,
		MaxHeaderBytes: opts.MaxHeaderBytes,
	}
}

func Wait(ctx context.Context, f func(ctx context.Context) error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sigChan
	c, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second*5))
	defer cancel()
	if err := f(c); err != nil {
		slog.Error("Error closing servers gracefully", slog.String("error", err.Error()))
	}
}
