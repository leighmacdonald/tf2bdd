package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/leighmacdonald/steamid/v4/steamid"
	"github.com/pkg/errors"
)

func main() {
	if err := run(); err != nil {
		slog.Error("error returned", slog.String("error", err.Error()))
		os.Exit(1)
	}

	os.Exit(0)
}

func run() error {
	steamKey := os.Getenv("STEAM_TOKEN")
	if steamKey == "" || len(steamKey) != 32 {
		return fmt.Errorf("invalid steam token: %s", steamKey)
	}

	if errSetKey := steamid.SetKey(steamKey); errSetKey != nil {
		return errSetKey
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return fmt.Errorf("invalid bot token: %s", token)
	}

	roles := strings.Split(os.Getenv("ROLES"), ",")
	if len(roles) == 0 {
		return errors.New("No discord roles defined, please set ROLES")
	}

	allowedRoles = roles

	database, errDatabase := openDB("./db.sqlite")
	if errDatabase != nil {
		return errDatabase
	}

	appCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if errSetupDB := setupDB(appCtx, database); errSetupDB != nil {
		return errSetupDB
	}

	httpServer := createHTTPServer(createRouter(database))

	discordBot, errBot := NewBot(token)
	if errBot != nil {
		return errBot
	}

	slog.Info("Add bot link", slog.String("link", discordAddURL()))

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Listener error", slog.String("error", err.Error()))
		}
	}()

	if errBotStart := startBot(appCtx, discordBot, database); errBotStart != nil {
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
