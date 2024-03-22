package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/leighmacdonald/steamid/v4/steamid"
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
	slog.Info("Connected to discord successfully")
}

func NewBot(app *App, token string) (*discordgo.Session, error) {
	dg, errDiscord := discordgo.New("Bot " + token)
	if errDiscord != nil {
		return nil, errors.Join(errDiscord, errors.New("dailed to create bot instance: %s"))
	}

	dg.AddHandler(ready)
	dg.AddHandler(app.messageCreate)
	dg.AddHandler(guildCreate)

	if errOpenDiscord := dg.Open(); errOpenDiscord != nil {
		return nil, errors.Join(errOpenDiscord, errors.New("could not connect to discord"))
	}

	return dg, nil
}

func memberHasRole(s *discordgo.Session, guildID string, userID string) (bool, error) {
	member, errMember := s.State.Member(guildID, userID)
	if errMember != nil {
		if member, errMember = s.GuildMember(guildID, userID); errMember != nil {
			return false, errMember
		}
	}
	for _, roleID := range member.Roles {
		role, errRole := s.State.Role(guildID, roleID)
		if errRole != nil {
			return false, errRole
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

func (a *App) add(s *discordgo.Session, m *discordgo.MessageCreate, sid steamid.SteamID, msg []string) error {
	a.idsMu.RLock()
	for _, existing := range a.ids {
		if existing.SteamID == sid {
			a.idsMu.RUnlock()
			return fmt.Errorf("duplicate steam id: %d", sid.Int64())
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
				return fmt.Errorf("unknown tag: %s", msg[i])
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
			return fmt.Errorf("duplicate steam id: %d", sid.Int64())
		} else {
			slog.Error("Failed to add player", slog.String("error", err.Error()))
			return fmt.Errorf("oops")
		}
	}

	a.idsMu.Lock()
	a.ids[player.SteamID] = player
	a.idsMu.Unlock()

	sendMsg(s, m, fmt.Sprintf("Added new entry successfully: %d", sid))

	return nil
}

func (a *App) check(s *discordgo.Session, m *discordgo.MessageCreate, sid steamid.SteamID) error {
	a.idsMu.RLock()
	_, found := a.ids[sid]
	a.idsMu.RUnlock()
	if !found {
		return fmt.Errorf("steam id does not exist in database: %d", sid.Int64())
	}
	sendMsg(s, m, fmt.Sprintf(":skull_crossbones: %d is a confirmed baddie :skull_crossbones: "+
		"https://steamcommunity.com/profiles/%d", sid.Int64(), sid.Int64()))
	return nil
}

func (a *App) steamid(s *discordgo.Session, m *discordgo.MessageCreate, sid steamid.SteamID) {
	var b strings.Builder
	b.WriteString("```")
	b.WriteString(fmt.Sprintf("Steam64: %d\n", sid.Int64()))
	b.WriteString(fmt.Sprintf("Steam32: %d\n", sid.AccountID))
	b.WriteString(fmt.Sprintf("Steam3:  %s", sid.Steam3()))
	b.WriteString("```")
	b.WriteString(fmt.Sprintf("Profile: <https://steamcommunity.com/profiles/%d>", sid.Int64()))
	sendMsg(s, m, b.String())
}

func (a *App) importJSON(s *discordgo.Session, m *discordgo.MessageCreate) error {
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
			return errors.Join(err, errors.New("failed to setup http request"))
		}

		resp, err := client.Do(req)
		if err != nil {
			return errors.Join(err, errors.New("failed to download file"))
		}
		_ = resp.Body.Close()

		var playerList masterListResp
		if errDecode := json.NewDecoder(resp.Body).Decode(&playerList); errDecode != nil {
			return errors.Join(errDecode, errors.New("failed to decode file"))
		}

		added += a.LoadMasterIDS(playerList.Players)
	}

	sendMsg(s, m, fmt.Sprintf("Loaded %d new players", added))

	return nil
}

func (a *App) del(s *discordgo.Session, m *discordgo.MessageCreate, sid steamid.SteamID) error {
	a.idsMu.RLock()
	_, found := a.ids[sid]
	a.idsMu.RUnlock()

	if !found {
		return fmt.Errorf("steam id does not exist in database: %d", sid.Int64())
	}

	if err := dropPlayer(a.ctx, a.db, sid); err != nil {
		return fmt.Errorf("error dropping player: %w", err)
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
		slog.Error("Failed to lookup role data")
		return
	}

	if !allowed && msg[0] != "!steamid" && msg[0] != "!count" {
		sendMsg(s, m, "Unauthorized")
		return
	}

	c, cancel := context.WithTimeout(a.ctx, time.Second*10)
	defer cancel()

	var sid steamid.SteamID
	if len(msg) > 1 {
		if strings.HasPrefix(msg[1], "http") {
			msg[1] = fmt.Sprintf("<%s>", msg[1])
			resolvedSid, errResolve := steamid.Resolve(c, msg[1])
			if errResolve != nil {
				sendMsg(s, m, fmt.Sprintf("Cannot resolve steam id: %s", msg[1]))

				return
			}
			sid = resolvedSid
		} else {
			sid = steamid.New(msg[1])
			if !sid.Valid() {
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
		slog.Error(`Failed to send message "%s": %s`, slog.String("msg", msg), slog.String("error", err.Error()))
	}
}

// This function will be called every time a new guild is joined.
func guildCreate(_ *discordgo.Session, event *discordgo.GuildCreate) {
	if event.Guild.Unavailable {
		return
	}
	for _, channel := range event.Guild.Channels {
		if channel.ID == event.Guild.ID {
			slog.Info("Connected to new guild", slog.String("guild", event.Guild.Name))
			return
		}
	}
}
