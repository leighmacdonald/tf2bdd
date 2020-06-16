package leagues

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"regexp"
	"sync"
	"tf2bdd/steamid"
)

type Division int

// *Rough* mapping of skill for each division for sorting, 0 being invite
const (
	RGLRankInvite    Division = 0
	ETF2LPremiership Division = 0

	UGCRankPlatinum Division = 1
	ETF2LDiv1       Division = 1
	RGLRankDiv1     Division = 1
	RGLRankDiv2     Division = 1

	ETF2LDiv2       Division = 2
	RGLRankMain     Division = 2
	RGLRankAdvanced Division = 2

	ETF2LMid    Division = 3
	UGCRankGold Division = 3

	ETF2LLow            Division = 4
	RGLRankIntermediate Division = 4

	ETF2LOpen        Division = 5
	RGLRankOpen      Division = 5
	UGCRankSilver    Division = 6
	UGCRankSteel     Division = 7
	UGCRankIron      Division = 8
	RGLRankFreshMeat Division = 9
	RGLRankNone      Division = 10
	UGCRankNone      Division = 10
)

type Season struct {
	League      string   `json:"league"`
	Division    string   `json:"division"`
	DivisionInt Division `json:"division_int"`
	Format      string   `json:"format"`
}

type LeagueQueryFunc func(ctx context.Context, steamid steamid.SID64) ([]Season, error)

var (
	reETF2L   *regexp.Regexp
	reUGCRank *regexp.Regexp
)

func get(ctx context.Context, url string, recv interface{}) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		// Don't follow redirects
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	r, err2 := client.Do(req)
	if err2 != nil {
		return nil, errors.Wrapf(err2, "error during get: %v", err2)
	}

	if recv != nil {
		body, err3 := ioutil.ReadAll(r.Body)
		if err3 != nil {
			return nil, errors.Wrapf(err, "error reading stream: %v", err3)
		}
		defer func() {
			if err := r.Body.Close(); err != nil {
				log.Errorf("Failed to close response body: %v", err)
			}
		}()
		if err := json.Unmarshal(body, &recv); err != nil {
			return r, errors.Wrapf(err, "Failed to decode json: %v", err)
		}
	}
	return r, nil
}

func getTF2Center(ctx context.Context, steamID steamid.SID64) ([]Season, error) {
	var s []Season
	r, err := get(ctx, fmt.Sprintf("https://tf2center.com/profile/%d", steamID), nil)
	if err != nil {
		return s, errors.Wrapf(err, "Failed to get tf2center history")
	}
	if r.StatusCode == http.StatusOK {
		s = append(s, Season{
			League:      "TF2Center",
			Division:    "PUG",
			DivisionInt: 9,
			Format:      "",
		})
	}
	return s, nil
}

func getOzFortress(steamID steamid.SID64) bool {
	r, err := http.Get(fmt.Sprintf("https://warzone.ozfortress.com/users/steam_id/%d", steamID))
	if err != nil {
		log.WithField("sid", steamID).Error("Failed to fetch ozfortress")
		return false
	}
	_ = r.Body.Close()
	return r.StatusCode == http.StatusOK
}

func FetchAll(ctx context.Context, steam steamid.SID64) []Season {
	var (
		wg      sync.WaitGroup
		results []Season
	)
	mu := &sync.RWMutex{}
	for _, f := range []LeagueQueryFunc{getRGL, getUGC, getTF2Center, getETF2L} {
		wg.Add(1)
		fn := f
		go func() {
			defer wg.Done()
			lHist, err := fn(ctx, steam)
			if err != nil {
				log.Warnf("Failed to get league data: %v", err)
			} else {
				mu.Lock()
				results = append(results, lHist...)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return results
}

func init() {
	reETF2L = regexp.MustCompile(`.org/forum/user/(\d+)`)
	reUGCRank = regexp.MustCompile(`Season (\d+) (\D+) (\S+)`)
}
