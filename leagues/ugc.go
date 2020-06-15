package leagues

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/leighmacdonald/steamid"
	"github.com/pkg/errors"
	"io/ioutil"
	"strings"
)

type UGCRank int

const (
	UGCRankNone     UGCRank = 0
	UGCRankIron     UGCRank = 1
	UGCRankSteel    UGCRank = 2
	UGCRankSilver   UGCRank = 3
	UGCRankGold     UGCRank = 4
	UGCRankPlatinum UGCRank = 5
)

const (
	ugcHLHeader = "TF2 Highlander Medals"
	ugc6sHeader = "TF2 6vs6 Medals"
	ugc4sHeader = "TF2 4vs4 Medals"
)

type ugcHist struct {
	UGCRank4s UGCRank `db:"ugc_rank_4s" json:"ugc_rank_4s"`
	UGCRank6s UGCRank `db:"ugc_rank_6s" json:"ugc_rank_6s"`
	UGCRankHL UGCRank `db:"ugc_rank_hl" json:"ugc_rank_hl"`
}

func getUGC(ctx context.Context, steam steamid.SID64) (LeagueHistory, error) {
	lHist := LeagueHistory{
		League: "UGC",
	}
	resp, err := get(ctx,
		fmt.Sprintf("https://www.ugcleague.com/players_page.cfm?player_id=%d", steam), nil)
	if err != nil {
		return lHist, errors.Wrapf(err, "Failed to get ugc response: %v", err)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return lHist, errors.Wrapf(err, "Failed to read response body: %v", err)
	}
	bodyStr := string(b)
	var hist ugcHist
	if err := parseUGCRank(bodyStr, &hist); err != nil {
		return lHist, errors.Wrapf(err, "Failed to parse ugc response: %v", err)
	}
	lHist.Exists = !strings.Contains(strings.ToLower(bodyStr), "no ugc tf2 league history")
	return lHist, nil
}

func parseUGCRank(body string, hist *ugcHist) error {
	dom, _ := goquery.NewDocumentFromReader(strings.NewReader(body))
	dom.Find("h5").Each(func(i int, selection *goquery.Selection) {
		text := selection.Text()
		if text == ugcHLHeader || text == ugc6sHeader || text == ugc4sHeader {
			ugcRank := UGCRankNone
			selection.Next().ChildrenFiltered("li").Each(func(i int, selection *goquery.Selection) {
				curRank := parseRankField(selection.Text())
				if curRank > ugcRank {
					ugcRank = curRank
				}
			})
			switch text {
			case ugcHLHeader:
				hist.UGCRankHL = ugcRank
			case ugc6sHeader:
				hist.UGCRank6s = ugcRank
			case ugc4sHeader:
				hist.UGCRank4s = ugcRank
			}
		}
	})
	return nil
}

func parseRankField(field string) UGCRank {
	ugcRank := UGCRankNone
	info := strings.Split(strings.Replace(field, "\n\n", "", -1), "\n")
	results := reUGCRank.FindStringSubmatch(info[0])
	if len(results) == 4 {
		curUGCRank := UGCRankNone
		switch results[3] {
		case "Platinum":
			curUGCRank = UGCRankPlatinum
		case "Gold":
			curUGCRank = UGCRankGold
		case "Silver":
			curUGCRank = UGCRankSilver
		case "Steel":
			curUGCRank = UGCRankSteel
		case "Iron":
			curUGCRank = UGCRankIron
		}
		if curUGCRank > ugcRank {
			ugcRank = curUGCRank
		}
	}
	return ugcRank
}
