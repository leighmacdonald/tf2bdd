package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/leighmacdonald/steamid"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const (
	perms    = 125952
	clientID = 720846565454905356
	addFmt   = "https://discord.com/oauth2/authorize?client_id=%d&scope=bot&permissions=%d"
)

var allowedRoles = []string{"717861254403981334", "493071295777341450"}

func AddUrl() string {
	return fmt.Sprintf(addFmt, clientID, perms)
}

// the "ready" event from Discord.
func ready(_ *discordgo.Session, _ *discordgo.Ready) {
	log.Infof("Connected to discord successfully")
}

func NewBot(app *App, token string) (*discordgo.Session, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create bot instance: %s", err)
	}
	dg.AddHandler(ready)
	dg.AddHandler(app.messageCreate)
	dg.AddHandler(guildCreate)
	if err := dg.Open(); err != nil {
		return nil, errors.Wrapf(err, "Could not connect to discord: %s", err)
	}
	return dg, nil
}

func memberHasRole(s *discordgo.Session, guildID string, userID string) (bool, error) {
	member, err := s.State.Member(guildID, userID)
	if err != nil {
		if member, err = s.GuildMember(guildID, userID); err != nil {
			return false, err
		}
	}
	for _, roleID := range member.Roles {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			return false, err
		}
		allowed := false
		for _, ar := range allowedRoles {
			if role.ID == ar {
				allowed = true
				break
			}
		}
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

func (a *App) count(s *discordgo.Session, m *discordgo.MessageCreate) {
	a.idsMu.RLock()
	counts := len(a.ids)
	a.idsMu.RUnlock()
	sendMsg(s, m, fmt.Sprintf("Total steamids tracked: %d", counts))
}

func (a *App) add(s *discordgo.Session, m *discordgo.MessageCreate, sid steamid.SID64, msg []string) error {
	a.idsMu.RLock()
	for _, existing := range a.ids {
		if existing.SteamID == sid {
			a.idsMu.RUnlock()
			return errors.Errorf("Duplicate steam id: %d", sid)
		}
	}
	a.idsMu.RUnlock()
	var attrs []Attributes
	if len(msg) == 2 {
		attrs = append(attrs, cheater)
	} else {
		for i := 2; i < len(msg); i++ {
			switch msg[i] {
			case "cheater":
				attrs = append(attrs, cheater)
			case "suspicious", "sus":
				attrs = append(attrs, sus)
			case "racist":
				attrs = append(attrs, racist)
			case "exploiter":
				attrs = append(attrs, exploiter)
			default:
				return errors.Errorf("Unknown tag: %s", msg[i])
			}
		}
	}
	player := Player{
		Attributes: attrs,
		LastSeen: LastSeen{
			Time: time.Now().Unix(),
		},
		SteamID: sid,
	}
	if err := addPlayer(a.ctx, a.db, player); err != nil {
		if err.Error() == "UNIQUE constraint failed: player.steamid" {
			return errors.Errorf("Duplicate steam id: %d", sid)
		} else {
			log.Errorf("Failed to add player: %v", err)
			return errors.Errorf("Oops")
		}
	}
	a.idsMu.Lock()
	a.ids[player.SteamID] = player
	a.idsMu.Unlock()
	sendMsg(s, m, fmt.Sprintf("Added new entry successfully: %d", sid))
	return nil
}
func (a *App) check(s *discordgo.Session, m *discordgo.MessageCreate, sid steamid.SID64) error {
	a.idsMu.RLock()
	_, found := a.ids[sid]
	a.idsMu.RUnlock()
	if !found {
		return errors.Errorf("Steam id doesnt exist in database: %d", sid)
	}
	sendMsg(s, m, fmt.Sprintf(":skull_crossbones: %d is a confirmed baddie :skull_crossbones: "+
		"http://steamcommunity.com/profiles/%d", sid.Int64(), sid.Int64()))
	return nil
}

func (a *App) steamid(s *discordgo.Session, m *discordgo.MessageCreate, sid steamid.SID64) {
	var b strings.Builder
	b.WriteString("```")
	b.WriteString(fmt.Sprintf("Steam64: %d\n", sid.Int64()))
	b.WriteString(fmt.Sprintf("Steam32: %d\n", steamid.SID64ToSID32(sid)))
	b.WriteString(fmt.Sprintf("Steam3:  %s", steamid.SID64ToSID3(sid)))
	b.WriteString("```")
	b.WriteString(fmt.Sprintf("Profile: <https://steamcommunity.com/profiles/%d>", sid.Int64()))
	sendMsg(s, m, b.String())
}

func (a *App) importJSON(s *discordgo.Session, m *discordgo.MessageCreate) error {
	log.Debugln(m)
	if len(m.Attachments) == 0 {
		return errors.New("Must attach json file to import")
	}
	client := http.Client{}
	c, cancel := context.WithTimeout(a.ctx, time.Second*30)
	defer cancel()
	added := 0
	for _, attach := range m.Attachments {
		req, err := http.NewRequestWithContext(c, "GET", attach.URL, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to setup http request")
		}
		resp, err := client.Do(req)
		if err != nil {
			return errors.Wrapf(err, "failed to download file")
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrapf(err, "failed to read response body")
		}
		_ = resp.Body.Close()
		var playerList masterListResp
		if err := json.Unmarshal(b, &playerList); err != nil {
			return errors.Wrapf(err, "failed to decode file")
		}
		added += a.LoadMasterIDS(playerList.Players)
	}
	sendMsg(s, m, fmt.Sprintf("Loaded %d new players", added))
	return nil
}

func (a *App) del(s *discordgo.Session, m *discordgo.MessageCreate, sid steamid.SID64) error {
	a.idsMu.RLock()
	_, found := a.ids[sid]
	a.idsMu.RUnlock()
	if !found {
		return errors.Errorf("Steam id doesnt exist in database: %d", sid)
	}
	if err := dropPlayer(a.ctx, a.db, sid); err != nil {
		return errors.Errorf("Error dropping player: %s", err.Error())
	}
	a.idsMu.Lock()
	delete(a.ids, sid)
	a.idsMu.Unlock()
	sendMsg(s, m, fmt.Sprintf("Dropped entry successfully: %d", sid))
	return nil
}

func (a *App) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	msg := strings.Split(strings.ToLower(m.Content), " ")
	minArgs := map[string]int{
		"!del":     2,
		"!check":   2,
		"!add":     2,
		"!steamid": 2,
		"!import":  1,
		"!count":   1,
	}
	count, found := minArgs[msg[0]]
	if !found {
		return
	}
	if len(msg) < count {
		sendMsg(s, m, fmt.Sprintf("Command requires at least %d args", count))
		return
	}
	allowed, err := memberHasRole(s, m.GuildID, m.Author.ID)
	if err != nil {
		log.Errorf("Failed to lookup role data")
		return
	}
	if !allowed && msg[0] != "!steamid" && msg[0] != "!count" {
		sendMsg(s, m, "Unauthorized")
		return
	}
	c, cancel := context.WithTimeout(a.ctx, time.Second*10)
	defer cancel()
	var sid steamid.SID64
	if len(msg) > 1 {
		if strings.HasPrefix(msg[1], "http") {
			msg[1] = fmt.Sprintf("<%s>", msg[1])
			sid, err = steamid.ResolveSID64(c, msg[1])
			if err != nil {
				sendMsg(s, m, fmt.Sprintf("Cannot resolve steam id: %s", msg[1]))
				return
			}
		} else {
			sid, err = steamid.StringToSID64(msg[1])
			if err != nil {
				sendMsg(s, m, fmt.Sprintf("Cannot resolve steam id: %s", msg[1]))
				return
			}
		}
		if !sid.Valid() {
			sendMsg(s, m, fmt.Sprintf("Cannot resolve steam id: %s", msg[1]))
			return
		}
	}

	var cmdErr error
	switch msg[0] {
	case "!del":
		cmdErr = a.del(s, m, sid)
	case "!check":
		cmdErr = a.check(s, m, sid)
	case "!add":
		cmdErr = a.add(s, m, sid, msg)
	case "!steamid":
		a.steamid(s, m, sid)
	case "!count":
		a.count(s, m)
	case "!import":
		cmdErr = a.importJSON(s, m)
	}
	if cmdErr != nil {
		sendMsg(s, m, cmdErr.Error())
	}
}

func sendMsg(s *discordgo.Session, m *discordgo.MessageCreate, msg string) {
	if _, err := s.ChannelMessageSend(m.ChannelID, msg); err != nil {
		log.Errorf(`Failed to send message "%s": %s`, msg, err)
	}
}

// This function will be called every time a new guild is joined.
func guildCreate(_ *discordgo.Session, event *discordgo.GuildCreate) {
	if event.Guild.Unavailable {
		return
	}
	for _, channel := range event.Guild.Channels {
		if channel.ID == event.Guild.ID {
			log.Infof("Connected to new guild: %s", event.Guild.Name)
			return
		}
	}
}
