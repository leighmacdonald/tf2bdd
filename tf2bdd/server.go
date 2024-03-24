package tf2bdd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/leighmacdonald/steamid/v4/steamid"
)

const schemaURL = "https://raw.githubusercontent.com/PazerOP/tf2_bot_detector/master/schemas/v3/playerlist.schema.json"

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
	Attributes []string        `json:"attributes"`
	LastSeen   LastSeen        `json:"last_seen,omitempty"`
	Author     int64           `json:"-"`
	CreatedOn  time.Time       `json:"-"`
	Proof      Proof           `json:"proof"`
}

func handleGetSteamIDs(database *sql.DB, config Config) http.HandlerFunc {
	hostPort := net.JoinHostPort(config.ListenHost, fmt.Sprintf("%d", config.ListenPort))
	updateURL := fmt.Sprintf("http://%s/v1/steamids", hostPort)
	if config.ExternalURL != "" {
		updateURL = fmt.Sprintf("%s/v1/steamids", strings.TrimSuffix(config.ExternalURL, "/"))
	}

	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		results := PlayerListRoot{
			ListSource: ListSource{
				Authors:     config.ListAuthors,
				Description: config.ListDescription,
				Title:       config.ListTitle,
				UpdateURL:   updateURL,
			},
			Schema:  schemaURL,
			Players: []Player{},
		}

		players, errPlayers := getPlayers(request.Context(), database)
		if errPlayers != nil {
			slog.Error("Failed to load players", slog.String("error", errPlayers.Error()))
			writer.WriteHeader(http.StatusInternalServerError)
			if errEncode := json.NewEncoder(writer).Encode(map[string]string{
				"error": "Could not load player list",
			}); errEncode != nil {
				slog.Error("failed to encode response", slog.String("error", errEncode.Error()))
			}

			return
		}

		if len(config.ExportedAttrs) > 0 {
			var filtered []Player

			for _, player := range players {
				for _, attr := range config.ExportedAttrs {
					if slices.Contains(player.Attributes, attr) {
						filtered = append(filtered, player)

						break
					}
				}
			}
			results.Players = filtered
		} else {
			results.Players = players
		}

		writer.WriteHeader(http.StatusOK)

		if errEncode := json.NewEncoder(writer).Encode(results); errEncode != nil {
			slog.Error("failed to encode response", slog.String("error", errEncode.Error()))
		}
	}
}

func CreateRouter(database *sql.DB, config Config) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/steamids", handleGetSteamIDs(database, config))

	return mux
}

func CreateHTTPServer(mux *http.ServeMux, listenAddr string) *http.Server {
	return &http.Server{
		Addr:           listenAddr,
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}
