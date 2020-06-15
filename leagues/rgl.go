package leagues

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"regexp"
	"tf2bdd/steamid"
)

type RGLRank int

const (
	RGLRankNone   RGLRank = 0
	RGLRankOpen   RGLRank = 1
	RGLRankDiv2   RGLRank = 2
	RGLRankDiv1   RGLRank = 3
	RGLRankInvite RGLRank = 4
)

type RGLDivision int

const (
	RGLFormatNone RGLDivision = 0
	RGLFormatPL   RGLDivision = 1
	RGLFormatHL   RGLDivision = 2
)

const (
	rglURL = "http://dsigafoo-001-site7.gtempurl.com/Public/API/v1/PlayerHistory.aspx?s=%d"
)

var (
	rglJSONRx *regexp.Regexp
)

func init() {
	rglJSONRx = regexp.MustCompile(`<span id="lblOutput">(.+?)<\/span>`)
}

type RGLResponse []struct {
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
	} `json:"PlayerHistory"`
}

func getRGL(ctx context.Context, steamid steamid.SID64) (LeagueHistory, error) {
	var lHist LeagueHistory
	resp, err := get(ctx, fmt.Sprintf(rglURL, steamid), nil)
	if err != nil {
		return lHist, err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return lHist, err
	}
	defer func() { _ = resp.Body.Close() }()

	m := rglJSONRx.FindStringSubmatch(string(b))
	if len(m) != 2 {
		return lHist, errors.Wrapf(err, "Failed to parse rgl span")
	}

	var hist RGLResponse
	if err := json.Unmarshal(b, &hist); err != nil {
		return lHist, errors.Wrapf(err, "Failed to parse rgl json")
	}
	if err := parseRGLRank(&hist); err != nil {
		return lHist, errors.New("failed to parse rgl history")
	}

	return lHist, nil
}

//
func parseRGLRank(hist *RGLResponse) error {
	return nil
}

//	dom, _ := goquery.NewDocumentFromReader(strings.NewReader(body))
//	dom.Find("tbody").Children().Each(func(i int, selection *goquery.Selection) {
//		if i == 0 {
//			// Skip header
//			return
//		}
//		curFormat := RGLFormatNone
//		curRank := RGLRankNone
//		selection.Children().Each(func(i int, selection *goquery.Selection) {
//			switch i {
//			case 0:
//				fmtTxt := strings.TrimSpace(selection.Text())
//				switch fmtTxt {
//				case "Prolander":
//					curFormat = RGLFormatPL
//				case "Highlander":
//					curFormat = RGLFormatHL
//				default:
//					curFormat = RGLFormatNone
//				}
//			case 3:
//				divTxt := strings.TrimSpace(selection.Text())
//				switch divTxt {
//				case "Invite":
//					curRank = RGLRankInvite
//				case "RGL-Invite":
//					curRank = RGLRankInvite
//				case "Div-1":
//					curRank = RGLRankDiv1
//				case "RGL Div-1":
//					curRank = RGLRankDiv1
//				case "Div-2 Red":
//					curRank = RGLRankDiv2
//				case "Div-2 Blue":
//					curRank = RGLRankDiv2
//				case "Open":
//					curRank = RGLRankOpen
//				default:
//					curRank = RGLRankNone
//				}
//				if curFormat > RGLFormatNone {
//					switch curFormat {
//					case RGLFormatPL:
//						if curRank > hist.RGLRankPL {
//							hist.RGLRankPL = curRank
//						}
//					case RGLFormatHL:
//						if curRank > hist.RGLRankHL {
//							hist.RGLRankHL = curRank
//						}
//					}
//				}
//			}
//		})
//	})
//}
