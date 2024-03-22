package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/leighmacdonald/steamid/v4/steamid"
)

const (
	perms  = 125952
	addFmt = "https://discord.com/oauth2/authorize?client_id=%s&scope=bot&permissions=%d"
)

var (
	allowedRoles []string
	clientID     string
)

func discordAddURL() string {
	return fmt.Sprintf(addFmt, clientID, perms)
}

// the "ready" event from Discord.
func ready(_ *discordgo.Session, _ *discordgo.Ready) {
	slog.Info("Connected to discord successfully")
}

func NewBot(token string) (*discordgo.Session, error) {
	dg, errDiscord := discordgo.New("Bot " + token)
	if errDiscord != nil {
		return nil, errors.Join(errDiscord, errors.New("failed to create bot instance: %s"))
	}

	return dg, nil
}

func startBot(ctx context.Context, dg *discordgo.Session, database *sql.DB) error {
	dg.AddHandler(ready)
	dg.AddHandler(messageCreate(ctx, database))
	dg.AddHandler(guildCreate)

	if errOpenDiscord := dg.Open(); errOpenDiscord != nil {
		return errors.Join(errOpenDiscord, errors.New("could not connect to discord"))
	}

	return nil
}

func memberHasRole(session *discordgo.Session, guildID string, userID string) (bool, error) {
	member, errMember := session.State.Member(guildID, userID)
	if errMember != nil {
		if member, errMember = session.GuildMember(guildID, userID); errMember != nil {
			return false, errMember
		}
	}
	for _, roleID := range member.Roles {
		role, errRole := session.State.Role(guildID, roleID)
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

func totalEntries(ctx context.Context, database *sql.DB) (string, error) {
	total, err := getCount(ctx, database)
	if err != nil {
		return "", fmt.Errorf("failed to get count: %w", err)
	}

	return fmt.Sprintf("Total steamids tracked: %d", total), nil
}

func addEntry(ctx context.Context, database *sql.DB, sid steamid.SteamID, msg []string, author int64) (string, error) {
	var attrs []string
	if len(msg) == 2 {
		attrs = append(attrs, "cheater")
	} else {
		for i := 2; i < len(msg); i++ {
			attrs = append(attrs, msg[i])
		}
	}

	player := Player{
		Attributes: attrs,
		LastSeen: LastSeen{
			Time: time.Now().Unix(),
		},
		SteamID: sid,
	}

	if err := addPlayer(ctx, database, player, author); err != nil {
		if err.Error() == "UNIQUE constraint failed: player.steamid" {
			return "", fmt.Errorf("duplicate steam id: %s", sid.String())
		}

		slog.Error("Failed to add player", slog.String("error", err.Error()))

		return "", fmt.Errorf("oops")
	}

	return fmt.Sprintf("Added new entry successfully: %s", sid.String()), nil
}

func checkEntry(ctx context.Context, database *sql.DB, sid steamid.SteamID) (string, error) {
	player, errPlayer := getPlayer(ctx, database, sid)
	if errPlayer != nil {
		return "", fmt.Errorf("steam id does not exist in database: %d", sid.Int64())
	}

	return fmt.Sprintf(":skull_crossbones: %s is a confirmed baddie :skull_crossbones: "+
		"https://steamcommunity.com/profiles/%d \nAuthor: <@%d>\nCreated: %s",
		player.LastSeen.PlayerName, sid.Int64(), player.Author, player.CreatedOn.String()), nil
}

func getSteamid(sid steamid.SteamID) string {
	var builder strings.Builder
	builder.WriteString("```")
	builder.WriteString(fmt.Sprintf("Steam64: %d\n", sid.Int64()))
	builder.WriteString(fmt.Sprintf("Steam32: %d\n", sid.AccountID))
	builder.WriteString(fmt.Sprintf("Steam3:  %s", sid.Steam3()))
	builder.WriteString("```")
	builder.WriteString(fmt.Sprintf("Profile: <https://steamcommunity.com/profiles/%d>", sid.Int64()))

	return builder.String()
}

func importJSON(ctx context.Context, database *sql.DB, message *discordgo.MessageCreate) (string, error) {
	if len(message.Attachments) == 0 {
		return "", errors.New("must attach json file to import")
	}

	client := &http.Client{}

	importCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	added := 0
	known, errKnown := getPlayers(importCtx, database)
	if errKnown != nil {
		return "", errors.Join(errKnown, errors.New("failed to load existing entries for comparison"))
	}

	author, errAuthor := strconv.ParseInt(message.Author.ID, 10, 64)
	if errAuthor != nil {
		return "", errors.New("failed to get discord author id")
	}

	for _, attach := range message.Attachments {
		newCount, errLoad := loadAttachment(importCtx, client, database, attach.URL, known, author)
		if errLoad != nil {
			return "", errLoad
		}

		added += newCount
	}

	return fmt.Sprintf("Loaded %d new players", added), nil
}

func loadAttachment(ctx context.Context, client *http.Client, database *sql.DB, url string, known []Player, author int64) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, errors.Join(err, errors.New("failed to setup http request"))
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, errors.Join(err, errors.New("failed to download file"))
	}

	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			slog.Error("failed to close body", slog.String("error", errClose.Error()))
		}
	}()

	var playerList PlayerListRoot
	if errDecode := json.NewDecoder(resp.Body).Decode(&playerList); errDecode != nil {
		slog.Error("error decoding", slog.String("error", errDecode.Error()))

		return 0, errors.Join(errDecode, errors.New("failed to decode file"))
	}

	var toAdd []Player
	for _, player := range playerList.Players {
		found := false
		for _, existing := range known {
			if player.SteamID.Int64() == existing.SteamID.Int64() {
				found = true

				break
			}
		}
		if !found {
			toAdd = append(toAdd, player)
		}
	}

	added := 0
	for _, p := range toAdd {
		if errAdd := addPlayer(ctx, database, p, author); errAdd != nil {
			slog.Error("failed to add new entry", slog.String("error", errAdd.Error()))

			continue
		}
		added++
	}

	return added, nil
}

func deleteEntry(ctx context.Context, database *sql.DB, sid steamid.SteamID) (string, error) {
	_, errPlayer := getPlayer(ctx, database, sid)
	if errPlayer != nil {
		return "", fmt.Errorf("steam id does not exist in database: %s", sid.String())
	}

	if err := dropPlayer(ctx, database, sid); err != nil {
		return "", fmt.Errorf("error dropping player: %w", err)
	}

	return fmt.Sprintf("Dropped entry successfully: %s", sid.String()), nil
}

func messageCreate(ctx context.Context, database *sql.DB) func(*discordgo.Session, *discordgo.MessageCreate) {
	return func(session *discordgo.Session, message *discordgo.MessageCreate) {
		// Ignore all messages created by the bot itself
		if message.Author.ID == session.State.User.ID {
			return
		}
		msg := strings.Split(strings.ToLower(message.Content), " ")
		minArgs := map[string]int{
			"!del":     2,
			"!check":   2,
			"!add":     2,
			"!steamid": 2,
			"!import":  1,
			"!count":   1,
		}

		argCount, found := minArgs[msg[0]]
		if !found {
			return
		}

		if len(msg) < argCount {
			sendMsg(session, message, fmt.Sprintf("Command requires at least %d args", argCount))

			return
		}

		allowed, err := memberHasRole(session, message.GuildID, message.Author.ID)
		if err != nil {
			slog.Error("Failed to lookup role data")

			return
		}

		if !allowed && msg[0] != "!steamid" && msg[0] != "!count" {
			sendMsg(session, message, "Unauthorized")

			return
		}

		var sid steamid.SteamID
		if len(msg) > 1 {
			resolveCtx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()

			userSid, errSid := steamid.Resolve(resolveCtx, msg[1])
			if errSid != nil {
				sendMsg(session, message, fmt.Sprintf("Cannot resolve steam id: %s", msg[1]))

				return
			} else if !userSid.Valid() {
				sendMsg(session, message, fmt.Sprintf("Invalid SteamID: %s", msg[1]))

				return
			}

			sid = userSid
		}

		var (
			response string
			cmdErr   error
		)

		switch strings.ToLower(msg[0]) {
		case "!del":
			response, cmdErr = deleteEntry(ctx, database, sid)
		case "!check":
			response, cmdErr = checkEntry(ctx, database, sid)
		case "!add":
			author, errAuthor := strconv.ParseInt(message.Author.ID, 10, 64)
			if errAuthor != nil {
				cmdErr = errors.New("failed to get discord author id")

				break
			}
			response, cmdErr = addEntry(ctx, database, sid, msg, author)
		case "!steamid":
			response = getSteamid(sid)
		case "!count":
			response, cmdErr = totalEntries(ctx, database)
		case "!import":
			response, cmdErr = importJSON(ctx, database, message)
		}

		if cmdErr != nil {
			sendMsg(session, message, cmdErr.Error())

			return
		}

		sendMsg(session, message, response)
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
