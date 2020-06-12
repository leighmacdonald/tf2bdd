package tf2bdd

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/leighmacdonald/steamid"
	"github.com/pkg/errors"
	"strings"
	"time"
)

const (
	perms    = 125952
	clientID = 720846565454905356
	addFmt   = "https://discord.com/oauth2/authorize?client_id=%d&scope=bot&permissions=%d"
)

var allowedRoles = []string{"717861254403981334"}

func AddUrl() string {
	return fmt.Sprintf(addFmt, clientID, perms)
}

// the "ready" event from Discord.
func ready(_ *discordgo.Session, _ *discordgo.Ready) {
	log.Infof("Connected to discord successfully")
}

func NewBot(token string) (*discordgo.Session, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create bot instance: %s", err)
	}
	dg.AddHandler(ready)
	dg.AddHandler(messageCreate)
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

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	msg := strings.Split(strings.ToLower(m.Content), " ")
	if len(msg) < 2 {
		return
	}
	if msg[0] != "!add" {
		return
	}
	allowed, err := memberHasRole(s, m.GuildID, m.Author.ID)
	if err != nil {
		log.Errorf("Failed to lookup role data")
		return
	}
	if !allowed {
		sendMsg(s, m, "Unauthorized")
		return
	}
	sid := steamid.ResolveSID64(msg[1])
	if !sid.Valid() {
		if strings.HasPrefix(msg[1], "http") {
			msg[1] = fmt.Sprintf("<%s>", msg[1])
		}
		sendMsg(s, m, fmt.Sprintf("Cannot resolve steam id: %s", msg[1]))
		return
	}
	idsMu.RLock()
	for _, existing := range ids {
		if existing.SteamID == sid {
			sendMsg(s, m, fmt.Sprintf("Duplicate steam id: %d", sid))
			idsMu.RUnlock()
			return
		}
	}
	idsMu.RUnlock()
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
			default:
				sendMsg(s, m, fmt.Sprintf("Unknown tag: %s", msg[i]))
				return
			}
		}
	}
	idsMu.Lock()
	ids = append(ids, Player{
		Attributes: attrs,
		LastSeen: LastSeen{
			Time: time.Now().Unix(),
		},
		SteamID: sid,
	})
	idsMu.Unlock()
	sendMsg(s, m, fmt.Sprintf("Added new entry successfully: %d", sid))
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
			log.Infof("Connected to new guild: %d", event.Guild.Name)
			return
		}
	}
}
