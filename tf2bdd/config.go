package tf2bdd

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var errConfigFile = errors.New("configuration file invalid")

type Config struct {
	SteamKey        string   `mapstructure:"steam_key"`
	DiscordClientID string   `mapstructure:"discord_client_id"`
	DiscordBotToken string   `mapstructure:"discord_bot_token"`
	DiscordRoles    []string `mapstructure:"discord_roles"`
	ExternalURL     string   `mapstructure:"external_url"`
	DatabasePath    string   `mapstructure:"database_path"`
	ListenHost      string   `mapstructure:"listen_host"`
	ListenPort      uint16   `mapstructure:"listen_port"`
	ListTitle       string   `mapstructure:"list_title"`
	ListDescription string   `mapstructure:"list_description"`
	ListAuthors     []string `mapstructure:"list_authors"`
	ExportedAttrs   []string `mapstructure:"exported_attrs"`
}

func (config Config) ListenAddr() string {
	return net.JoinHostPort(config.ListenHost, fmt.Sprintf("%d", config.ListenPort))
}

func (config Config) UpdateURL() (string, error) {
	extURL := config.ExternalURL
	if extURL == "" {
		host := config.ListenHost
		if host == "" {
			host = "localhost"
		}
		extURL = fmt.Sprintf("http://%s", net.JoinHostPort(host, fmt.Sprintf("%d", config.ListenPort)))
	}

	parsed, errParse := url.Parse(extURL)
	if errParse != nil {
		return "", errParse
	}
	parsed.Path = "/v1/steamids"

	return parsed.String(), nil
}

func ReadConfig() (Config, error) {
	if home, errHomeDir := homedir.Dir(); errHomeDir != nil {
		viper.AddConfigPath(home)
	}

	viper.AddConfigPath(".")
	viper.SetConfigName("tf2bdd")
	viper.SetConfigType("yml")
	viper.SetEnvPrefix("tf2bdd")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	defaultValues := map[string]any{
		"steam_key":         "",
		"discord_client_id": "",
		"discord_bot_token": "",
		"discord_roles":     []string{},
		"external_url":      "",
		"database_path":     "./db.sqlite",
		"listen_host":       "localhost",
		"listen_port":       8899,
		"list_title":        "",
		"list_description":  "",
		"list_authors":      []string{"anonymous"},
		"exported_attrs":    []string{},
	}

	for configKey, value := range defaultValues {
		viper.SetDefault(configKey, value)
	}

	if errReadConfig := viper.ReadInConfig(); errReadConfig != nil {
		return Config{}, errors.Join(errReadConfig, errConfigFile)
	}

	var config Config
	if errUnmarshal := viper.Unmarshal(&config); errUnmarshal != nil {
		return config, errUnmarshal
	}

	return config, nil
}

func ValidateConfig(config Config) error {
	if config.SteamKey == "" || len(config.SteamKey) != 32 {
		return fmt.Errorf("invalid steam token: %s", config.SteamKey)
	}

	if config.DiscordClientID == "" {
		return errors.New("discord client_id not set")
	}

	if config.DiscordBotToken == "" {
		return errors.New("discord bot token not set")
	}

	if len(config.DiscordRoles) == 0 {
		return errors.New("no discord roles are defined")
	}

	if len(config.ListTitle) == 0 {
		return errors.New("list_title cannot be empty")
	}

	if len(config.ListDescription) == 0 {
		return errors.New("list_description cannot be empty")
	}

	return nil
}
