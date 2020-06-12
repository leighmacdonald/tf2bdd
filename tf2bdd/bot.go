package tf2bdd

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

const (
	perms    = 125952
	clientID = 720846565454905356
	addFmt   = "https://discord.com/oauth2/authorize?client_id=%d&scope=bot&permissions=%d"
)

func AddUrl() string {
	return fmt.Sprintf(addFmt, clientID, perms)
}

// the "ready" event from Discord.
func ready(s *discordgo.Session, event *discordgo.Ready) {
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

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}
}

// This function will be called every time a new guild is joined.
func guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
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
