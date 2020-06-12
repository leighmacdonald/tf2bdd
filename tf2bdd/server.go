package tf2bdd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/leighmacdonald/steamid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/toorop/gin-logrus"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Attributes string

const (
	cheater   Attributes = "cheater"
	racist    Attributes = "racist"
	sus       Attributes = "suspicious"
	exploiter Attributes = "exploiter"
)

var (
	authKeys []string
	ids      []Player
	idsMu    *sync.RWMutex
	log      *logrus.Logger
)

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
	Attributes []Attributes  `json:"attributes"`
	LastSeen   LastSeen      `json:"last_seen,omitempty"`
	SteamID    steamid.SID64 `json:"steam_id"`
}

type ErrResp struct {
	Error string `json:"error"`
}

type SucResp struct {
	Message string `json:"message"`
}

func handleGetSteamIDS(c *gin.Context) {
	idsMu.RLock()
	steamIDs := ids
	idsMu.RUnlock()
	c.JSON(200, steamIDs)
}

func handleAddSteamID(c *gin.Context) {
	var req AddSteamIDReq
	if err := c.BindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrResp{Error: "Invalid request format"})
		return
	}
	steamID := steamid.StringToSID64(req.SteamID)
	if !steamID.Valid() {
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrResp{
			Error: fmt.Sprintf("Invalid steam id: %s", req.SteamID)})
		return
	}
	idsMu.Lock()
	ids = append(ids, Player{
		Attributes: req.Attributes,
		SteamID:    steamID,
	})
	idsMu.Unlock()
	c.JSON(200, SucResp{Message: fmt.Sprintf("Added successfully: %s", req.SteamID)})
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		isBaddie := true
		authKey := c.GetHeader("Authorization")
		for _, k := range authKeys {
			if k == authKey {
				isBaddie = false
				break
			}
		}
		if isBaddie {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrResp{Error: "Hi telegram."})
			return
		}
		c.Next()
	}
}

const masterList = "https://raw.githubusercontent.com/PazerOP/tf2_bot_detector/master/tf2_bot_detector/cfg/playerlist.official.json"

type masterListResp struct {
	Schema  string     `json:"$schema"`
	Players []MLPlayer `json:"players"`
	Version int        `json:"version"`
}

type MLPlayer struct {
	Attributes []Attributes `json:"attributes"`
	LastSeen   LastSeen     `json:"last_seen,omitempty"`
	SteamID    string       `json:"steamid"`
}

func DownloadMasterList() ([]Player, error) {
	resp, err := http.Get(masterList)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get master list: %s", err)
	}
	if resp.StatusCode != 200 {
		return nil, errors.Wrapf(err, "Invalid status code from gh: %d", resp.StatusCode)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read response body: %d", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Errorf("Failed to close response body: %s", err)
		}
	}()
	var listResp masterListResp
	if err := json.Unmarshal(b, &listResp); err != nil {
		return nil, errors.Wrapf(err, "Failed to decide master list: %s", err)
	}
	var p []Player
	for _, mlP := range listResp.Players {
		newPlayer := Player{
			Attributes: mlP.Attributes,
			LastSeen:   mlP.LastSeen,
			SteamID:    steamid.StringToSID64(mlP.SteamID),
		}
		if newPlayer.SteamID.Valid() {
			p = append(p, newPlayer)
		}
	}
	return p, nil
}

func LoadMasterIDS() {
	ml, err := DownloadMasterList()
	if err != nil {
		log.Errorf("Failed to download master list from GH: %s", err)
		return
	}
	idsMu.Lock()
	for _, p := range ml {
		ids = append(ids, p)
	}
	idsMu.Unlock()
	log.Infof("Downloaded %d steamids", len(ml))
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
		ListenAddr:     ":27015",
		UseTLS:         false,
		Handler:        nil,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      nil,
	}
}

func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(ginlogrus.Logger(log))
	r.GET("/v1/steamids", handleGetSteamIDS)
	authed := r.Group("/", AuthRequired())
	authed.POST("/v1/steamids", handleAddSteamID)
	return r
}

func NewHTTPServer(opts *HTTPOpts) *http.Server {
	var tlsCfg *tls.Config
	if opts.UseTLS && opts.TLSConfig == nil {
		tlsCfg = &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}
	} else {
		tlsCfg = nil
	}
	srv := http.Server{
		Addr:           opts.ListenAddr,
		Handler:        opts.Handler,
		TLSConfig:      tlsCfg,
		ReadTimeout:    opts.ReadTimeout,
		WriteTimeout:   opts.WriteTimeout,
		MaxHeaderBytes: opts.MaxHeaderBytes,
	}
	return &srv
}

func Wait(ctx context.Context, f func(ctx context.Context) error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sigChan
	c, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second*5))
	defer cancel()
	if err := f(c); err != nil {
		log.Errorf("Error closing servers gracefully; %s", err)
	}
}

func init() {
	log = logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		ForceColors: true,
	})
	idsMu = &sync.RWMutex{}
}
