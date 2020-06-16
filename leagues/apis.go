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
	"time"
)

type CompHist struct {
	SteamID    steamid.SID64 `db:"steam_id" json:"-"`
	RGLRankHL  RGLDivision   `db:"rgl_rank_hl" json:"rgl_rank_hl"`
	RGLRankPL  RGLDivision   `db:"rgl_rank_pl" json:rgl_rank_pl"`
	UGCRank4s  UGCRank       `db:"ugc_rank_4s" json:"ugc_rank_4s"`
	UGCRank6s  UGCRank       `db:"ugc_rank_6s" json:"ugc_rank_6s"`
	UGCRankHL  UGCRank       `db:"ugc_rank_hl" json:"ugc_rank_hl"`
	ESEARank6s ESEARank      `db:"esea_rank_6s" json:"esea_rank_6s"`
	UpdatedOn  time.Time     `db:"updated_on" json:"updated_on"`
}

type LeagueHistory struct {
	Exists       bool   `json:"exists"`
	League       string `json:"league"`
	MaxDivision  string `json:"max_division"`
	LastDivision string `json:"last_division"`
}

type LeagueQueryFunc func(ctx context.Context, steamid steamid.SID64) (LeagueHistory, error)

var (
	reETF2L   *regexp.Regexp
	reUGCRank *regexp.Regexp
)

func get(ctx context.Context, url string, recv interface{}) (*http.Response, error) {
	c, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	req, err := http.NewRequestWithContext(c, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create request: %v", err)
	}
	client := &http.Client{
		// Don't follow redirects
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	r, err2 := client.Do(req)
	if err2 != nil {
		return nil, errors.Wrapf(err, "error during get: %v", err2)
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Errorf("Failed to close response body: %v", err)
		}
	}()
	body, err3 := ioutil.ReadAll(r.Body)
	if err3 != nil {
		return nil, errors.Wrapf(err, "error reading stream: %v", err3)
	}
	if recv != nil {
		if err := json.Unmarshal(body, &recv); err != nil {
			return r, errors.Wrapf(err, "Failed to decode json: %v", err)
		}
	}
	return r, nil
}

func getTF2Center(ctx context.Context, steamID steamid.SID64) (LeagueHistory, error) {
	lHist := LeagueHistory{
		League: "TF2Center",
	}
	r, err := get(ctx, fmt.Sprintf("https://tf2center.com/profile/%d", steamID), nil)
	if err != nil {
		return lHist, errors.Wrapf(err, "Failed to get tf2center history")
	}
	defer func() {
		_ = r.Body.Close()
	}()
	lHist.Exists = r.StatusCode == http.StatusOK
	return lHist, nil
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

func FetchAll(ctx context.Context, steam steamid.SID64) []LeagueHistory {
	var (
		wg      sync.WaitGroup
		results []LeagueHistory
	)
	mu := &sync.RWMutex{}
	c, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	for _, f := range []LeagueQueryFunc{getRGL, getUGC, getTF2Center, getESEA} {
		wg.Add(1)
		fn := f
		go func() {
			defer wg.Done()
			lHist, err := fn(c, steam)
			if err != nil {
				log.Warnf("Failed to get league data: %v", err)
			}
			mu.Lock()
			results = append(results, lHist)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return results
}

func init() {
	reETF2L = regexp.MustCompile(`.org/forum/user/(\d+)`)
	reUGCRank = regexp.MustCompile(`Season (\d+) (\D+) (\S+)`)
}
