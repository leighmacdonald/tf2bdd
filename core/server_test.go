package core

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/leighmacdonald/steamid"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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
	return NewApp(context.Background(), ":memory:", []string{testAuthKey})
}

func TestHandleAddSteamID(t *testing.T) {
	testID := steamid.SID64(76561198003911389)
	b, err := json.Marshal(&AddSteamIDReq{
		Attributes: []Attributes{cheater, racist},
		SteamID:    testID.String(),
		Username:   "user_who_added",
	})
	if err != nil {
		t.Fatalf("Failed to marshal test body")
	}
	req, err := http.NewRequest("POST", "/v1/steamids", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", testAuthKey)
	w := httptest.NewRecorder()
	app, err := newTestApp()
	require.NoError(t, err)
	oldCount := len(app.ids)
	NewRouter(app).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, oldCount+1, len(app.ids))
	match := false
	var matched Player
	for _, p := range app.ids {
		if p.SteamID == testID {
			matched = p
			match = true
			break
		}
	}
	require.True(t, match)
	require.Equal(t, []Attributes{cheater, racist}, matched.Attributes)
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
