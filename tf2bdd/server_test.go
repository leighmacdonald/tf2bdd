package tf2bdd_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/leighmacdonald/steamid/v4/steamid"
	"github.com/leighmacdonald/tf2bdd/tf2bdd"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/require"
)

func newTestDB(ctx context.Context) (*sql.DB, error) {
	db, errDB := tf2bdd.OpenDB(":memory:")
	if errDB != nil {
		return nil, errDB
	}

	return db, tf2bdd.SetupDB(ctx, db)
}

func TestHandleGetSteamIDS(t *testing.T) {
	testConfig := tf2bdd.Config{
		SteamKey:        "",
		DiscordClientID: "",
		DiscordBotToken: "",
		DiscordRoles:    nil,
		ExternalURL:     "https://example.com/",
		DatabasePath:    "",
		ListenHost:      "",
		ListenPort:      0,
		ListTitle:       "test title",
		ListDescription: "test description",
		ListAuthors:     []string{"test author"},
		ExportedAttrs:   nil,
	}
	ctx := context.Background()

	req, errReq := http.NewRequestWithContext(ctx, http.MethodGet, "/v1/steamids", nil)
	if errReq != nil {
		t.Fatal(errReq)
	}

	recorder := httptest.NewRecorder()
	database, errApp := newTestDB(ctx)
	require.NoError(t, errApp)

	localPlayers := []tf2bdd.Player{
		{
			SteamID:    steamid.New(76561198237337976),
			Attributes: []string{"cheater"},
			LastSeen:   tf2bdd.LastSeen{},
		},
		{
			SteamID:    steamid.New(76561198834913692),
			Attributes: []string{"cheater"},
			LastSeen:   tf2bdd.LastSeen{},
		},
	}

	for _, p := range localPlayers {
		require.NoError(t, tf2bdd.AddPlayer(ctx, database, p, 0))
	}

	tf2bdd.CreateRouter(database, testConfig).ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var players tf2bdd.PlayerListRoot
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&players))
	require.Equal(t, len(localPlayers), len(players.Players))
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
