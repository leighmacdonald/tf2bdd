package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/leighmacdonald/steamid/v4/steamid"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDownloadMasterList(t *testing.T) {
	_, err := downloadMasterList()
	require.NoError(t, err)
}

func newTestDB(ctx context.Context) (*sql.DB, error) {
	return openDB(ctx, ":memory:")
}

func TestHandleGetSteamIDS(t *testing.T) {
	ctx := context.Background()
	req, errReq := http.NewRequest("GET", "/v1/steamids", nil)
	if errReq != nil {
		t.Fatal(errReq)
	}
	w := httptest.NewRecorder()
	database, errApp := newTestDB(ctx)
	require.NoError(t, errApp)

	localPlayers := []Player{
		{
			SteamID:    steamid.New(76561198237337976),
			Attributes: []Attributes{cheater},
			LastSeen:   LastSeen{},
		},
		{
			SteamID:    steamid.New(76561198834913692),
			Attributes: []Attributes{cheater},
			LastSeen:   LastSeen{},
		},
	}
	for _, p := range localPlayers {
		require.NoError(t, addPlayer(ctx, database, p))
	}

	createRouter(database).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var players PlayerListRoot
	require.NoError(t, json.NewDecoder(w.Body).Decode(&players))
	require.Equal(t, len(localPlayers), len(players.Players))
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
