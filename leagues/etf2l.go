package leagues

import "github.com/leighmacdonald/steamid"

const (
	baseURL = "https://api.etf2l.org"
)

type ETF2LPlayer struct {
	Player struct {
		Bans       interface{} `json:"bans"`
		Classes    []string    `json:"classes"`
		Country    string      `json:"country"`
		ID         int         `json:"id"`
		Name       string      `json:"name"`
		Registered int         `json:"registered"`
		Steam      struct {
			Avatar string `json:"avatar"`
			ID     string `json:"id"`
			ID3    string `json:"id3"`
			ID64   string `json:"id64"`
		} `json:"steam"`
		Teams []struct {
			Competitions struct {
				Comp struct {
					Category    string `json:"category"`
					Competition string `json:"competition"`
					Division    struct {
						Name interface{} `json:"name"`
						Tier interface{} `json:"tier"`
					} `json:"division"`
					URL string `json:"url"`
				} `json:"comp"`
			} `json:"competitions,omitempty"`
			Country  string `json:"country"`
			Homepage string `json:"homepage"`
			ID       int    `json:"id"`
			Irc      struct {
				Channel interface{} `json:"channel"`
				Network interface{} `json:"network"`
			} `json:"irc"`
			Name   string `json:"name"`
			Server string `json:"server"`
			Steam  struct {
				Avatar string `json:"avatar"`
				Group  string `json:"group"`
			} `json:"steam"`
			Tag  string `json:"tag"`
			Type string `json:"type"`
			Urls struct {
				Matches   string `json:"matches"`
				Results   string `json:"results"`
				Self      string `json:"self"`
				Transfers string `json:"transfers"`
			} `json:"urls"`
		} `json:"teams"`
		Title string `json:"title"`
		Urls  struct {
			Results   string `json:"results"`
			Self      string `json:"self"`
			Transfers string `json:"transfers"`
		} `json:"urls"`
	} `json:"player"`
	Status struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
}

func getETF2L(steamid steamid.SID64) (LeagueHistory, error) {
	return LeagueHistory{}, nil
}
