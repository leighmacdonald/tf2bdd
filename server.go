package main

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"errors"
	"github.com/leighmacdonald/steamid/v4/steamid"
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
	description = "Curated list of steamids created by the @trusted people in the official discord server"
	updateURL   = "https://trusted.roto.lol/v1/steamids"
)

type ListSource struct {
	Authors     []string `json:"authors"`
	Description string   `json:"description"`
	Title       string   `json:"title"`
	UpdateURL   string   `json:"update_url"`
}

type PlayerListRoot struct {
	ListSource ListSource `json:"file_info"`
	Schema     string     `json:"$schema"`
	Players    []Player   `json:"players"`
	Version    int        `json:"version"`
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

func handleGetSteamIDS(database *sql.DB) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		results := PlayerListRoot{
			ListSource: ListSource{
				Authors:     []string{"@trusted"},
				Description: description,
				Title:       title,
				UpdateURL:   updateURL,
			},
			Schema:  schemaURL,
			Players: []Player{},
		}

		players, errPlayers := getPlayers(request.Context(), database)
		if errPlayers != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte("error"))

			return
		}
		results.Players = players
		writer.WriteHeader(http.StatusOK)
		if errEncode := json.NewEncoder(writer).Encode(results); errEncode != nil {
			slog.Error("failed to encode response", slog.String("error", errEncode.Error()))
		}
	}
}

func downloadMasterList() ([]Player, error) {
	resp, err := http.Get(masterList)
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get master list"))
	}
	if resp.StatusCode != 200 {
		return nil, errors.Join(err, errors.New("invalid status code from gh"))
	}

	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			slog.Error("Failed to close response body", slog.String("error", errClose.Error()))
		}
	}()

	var listResp PlayerListRoot
	if errDecode := json.NewDecoder(resp.Body).Decode(&listResp); errDecode != nil {
		return nil, errors.Join(errDecode, errors.New("failed to decode master list"))
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

func createRouter(database *sql.DB) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/steamids", handleGetSteamIDS(database))

	return mux
}

func createHTTPServer(mux *http.ServeMux) *http.Server {
	return &http.Server{
		Addr:           ":8899",
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}
