package leagues

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"regexp"
	"sort"
	"strings"
	"tf2bdd/steamid"
)

const (
	rglURL = "http://dsigafoo-001-site7.gtempurl.com/Public/API/v1/PlayerHistory.aspx?s=%d"
)

var (
	rglJSONRx *regexp.Regexp
)

func init() {
	rglJSONRx = regexp.MustCompile(`<span id="lblOutput">(.+?)</span>`)
}

type rglPlayer struct {
	SteamID       string `json:"SteamId"`
	CurrentAlias  string `json:"CurrentAlias"`
	PlayerHistory []struct {
		SteamID      string      `json:"SteamId"`
		CurrentAlias string      `json:"CurrentAlias"`
		TeamID       int         `json:"TeamId"`
		TeamName     string      `json:"TeamName"`
		DivisionID   int         `json:"DivisionId"`
		DivisionName string      `json:"DivisionName"`
		GroupID      int         `json:"GroupId"`
		GroupName    interface{} `json:"GroupName"`
		SeasonID     int         `json:"SeasonId"`
		SeasonName   string      `json:"SeasonName"`
		RegionID     int         `json:"RegionId"`
		RegionName   string      `json:"RegionName"`
		RegionURL    string      `json:"RegionURL"`
		RegionFormat string      `json:"RegionFormat"`
		StartDate    string      `json:"StartDate"`
		EndDate      string      `json:"EndDate"`
		Wins         int         `json:"Wins"`
		Loses        int         `json:"Loses"`
		AmtWon       interface{} `json:"AmtWon"`
		EndRank      interface{} `json:"EndRank"`
	}
}

func getRGL(ctx context.Context, steamid steamid.SID64) ([]Season, error) {
	var seasons []Season
	resp, err := get(ctx, fmt.Sprintf(rglURL, steamid), nil)
	if err != nil {
		return seasons, err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return seasons, err
	}
	defer func() { _ = resp.Body.Close() }()

	m := rglJSONRx.FindStringSubmatch(string(b))
	if len(m) != 2 {
		return seasons, errors.Wrapf(err, "Failed to parse rgl span")
	}
	seasons, err = parseRGL(bytes.NewBufferString(m[1]).Bytes())
	if err != nil {
		return seasons, errors.New("failed to parse rgl history")
	}
	return seasons, nil
}

//
func parseRGL(b []byte) ([]Season, error) {
	var hist []rglPlayer
	var seasons []Season
	if err := json.Unmarshal(b, &hist); err != nil {
		return seasons, errors.Wrapf(err, "Failed to parse rgl json")
	}
	for _, l := range hist {
		for _, h := range l.PlayerHistory {
			var s Season
			div, divStr := parseRGLDivision(h.DivisionName)
			if div == RGLRankNone {
				continue
			}
			formatStr := parseRGLFormat(h.RegionFormat)
			if formatStr == "" {
				continue
			}
			s.Division = divStr
			s.DivisionInt = div
			s.Format = formatStr
			seasons = append(seasons, s)
		}
		sort.Slice(seasons, func(i, j int) bool {
			return seasons[i].DivisionInt < seasons[j].DivisionInt
		})
	}
	return seasons, nil
}

func parseRGLDivision(div string) (Division, string) {
	switch strings.ToLower(div) {
	case "invite", "rgl-invite":
		return RGLRankInvite, "invite"
	case "div-1", "rgl div-1":
		return RGLRankDiv1, "Div-1"
	case "div-2 red", "div-2 blue":
		return RGLRankDiv2, "Div-2"
	case "open":
		return RGLRankOpen, "Open"
	case "intermediate":
		return RGLRankIntermediate, "Intermediate"
	case "advanced":
		return RGLRankAdvanced, "Advanced"
	case "main":
		return RGLRankMain, "Main"
	case "dead teams", "admin placement", "unready", "fresh meat", "one day cup":
		fallthrough
	default:
		return RGLRankNone, ""
	}
}

func parseRGLFormat(f string) string {
	switch strings.ToLower(f) {
	case "prolander":
		return "Prolander"
	case "highlander":
		return "Highlander"
	case "trad. sixes":
		return "6s"
	case "nr sixes":
		return "NR6s"
	}
	return ""
}
