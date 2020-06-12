package tf2bdd

import (
	"bytes"
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
	l, err := DownloadMasterList()
	require.NoError(t, err)
	require.Greater(t, len(l), 10)
}

func TestHandleAddSteamIDBadAuth(t *testing.T) {
	// Bad auth
	reqBadAuth, err := http.NewRequest("POST", "/v1/steamids", nil)
	if err != nil {
		t.Fatal(err)
	}
	reqBadAuth.Header.Set("Authorization", "asdfasdf")
	w2 := httptest.NewRecorder()
	NewRouter().ServeHTTP(w2, reqBadAuth)
	require.Equal(t, http.StatusUnauthorized, w2.Code)
}

func TestHandleAddSteamID(t *testing.T) {
	idCount := len(ids)
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
	NewRouter().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, idCount+1, len(ids))
	match := false
	var matched Player
	for _, p := range ids {
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
	NewRouter().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var players []Player
	b, err := ioutil.ReadAll(w.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &players))
	require.Equal(t, len(ids), len(players))
}

func TestMain(m *testing.M) {
	idsMu.Lock()
	authKeys = append(authKeys, testAuthKey)
	ids = []Player{
		{
			Attributes: []Attributes{cheater},
			LastSeen:   LastSeen{},
			SteamID:    76561197966480940,
		},
		{
			Attributes: []Attributes{cheater},
			LastSeen: LastSeen{
				PlayerName: "poopyhead‏",
				Time:       1591238458,
			},
			SteamID: 76561197992466050,
		},
		{
			Attributes: []Attributes{cheater},
			LastSeen:   LastSeen{},
			SteamID:    76561197972191700,
		},
	}
	idsMu.Unlock()
	os.Exit(m.Run())
}
