package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/leighmacdonald/tf2bdd/tf2bdd"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leighmacdonald/steamid/v4/steamid"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("error returned", slog.String("error", err.Error()))
		os.Exit(1)
	}

	os.Exit(0)
}

func run() error {
	slog.Info("Starting tf2bdd", slog.String("version", version))

	config, errConfig := tf2bdd.ReadConfig()
	if errConfig != nil {
		return errConfig
	}

	if errValidate := tf2bdd.ValidateConfig(config); errValidate != nil {
		return fmt.Errorf("config file validation error: %w", errValidate)
	}

	if errSetKey := steamid.SetKey(config.SteamKey); errSetKey != nil {
		return errSetKey
	}

	database, errDatabase := tf2bdd.OpenDB(config.DatabasePath)
	if errDatabase != nil {
		return errDatabase
	}

	appCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if errSetupDB := tf2bdd.SetupDB(appCtx, database); errSetupDB != nil {
		return errSetupDB
	}

	listenAddr := fmt.Sprintf("%s:%d", config.ListenHost, config.ListenPort)
	httpServer := tf2bdd.CreateHTTPServer(tf2bdd.CreateRouter(database, config), listenAddr)

	discordBot, errBot := tf2bdd.NewBot(config.DiscordBotToken)
	if errBot != nil {
		return errBot
	}

	slog.Info("Add bot", slog.String("link", tf2bdd.DiscordAddURL(config.DiscordClientID)))
	slog.Info("Make sure you enable \"Message Content Intent\" on your discord config under the Bot settings via discord website")
	slog.Info("Listening on", slog.String("addr", fmt.Sprintf("http://%s", listenAddr)))

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Listener error", slog.String("error", err.Error()))
		}
	}()

	if errBotStart := tf2bdd.StartBot(appCtx, discordBot, database, config); errBotStart != nil {
		slog.Error("discord bot error", slog.String("error", errBotStart.Error()))
	}

	<-appCtx.Done()

	slog.Info("Shutting down")

	if err := discordBot.Close(); err != nil {
		slog.Error("Failed to properly shutdown discord client", slog.String("error", err.Error()))
	}

	cancelCtx, cancelHTTP := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancelHTTP()

	if err := httpServer.Shutdown(cancelCtx); err != nil {
		slog.Error("Failed to cleanly shutdown http service: %s", slog.String("error", err.Error()))
	}

	return nil
}
