package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const testAuthKey = "123456"

func TestDownloadMasterList(t *testing.T) {
	_, err := DownloadMasterList()
	require.NoError(t, err)
}

func TestHandleAddSteamIDBadAuth(t *testing.T) {
	// Bad auth
	reqBadAuth, err := http.NewRequest("POST", "/v1/steamids", nil)
	if err != nil {
		t.Fatal(err)
	}
	reqBadAuth.Header.Set("Authorization", "asdfasdf")
	w2 := httptest.NewRecorder()
	a, err := newTestApp()
	require.NoError(t, err)
	NewRouter(a).ServeHTTP(w2, reqBadAuth)
	require.Equal(t, http.StatusUnauthorized, w2.Code)
}

func newTestApp() (*App, error) {
	return NewApp(context.Background(), ":memory:")
}

func TestHandleGetSteamIDS(t *testing.T) {
	req, err := http.NewRequest("GET", "/v1/steamids", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	app, err := newTestApp()
	require.NoError(t, err)
	NewRouter(app).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var players masterListResp
	b, err := ioutil.ReadAll(w.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &players))
	require.Equal(t, len(app.ids), len(players.Players))
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
