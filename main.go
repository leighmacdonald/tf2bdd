package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"errors"
	"github.com/leighmacdonald/steamid/v4/steamid"
)

func main() {
	if err := run(); err != nil {
		slog.Error("error returned", slog.String("error", err.Error()))
		os.Exit(1)
	}

	os.Exit(0)
}

func run() error {
	ctx := context.Background()
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

	database, errDatabase := openDB(ctx, "./db.sqlite")
	if errDatabase != nil {
		return errDatabase
	}

	ml, errApp := downloadMasterList()
	if errApp != nil {
		slog.Warn("Failed to download master list from GH", slog.String("error", errApp.Error()))
	}

	// TODO remove master entries

	own, errOwn := getPlayers(ctx, database)
	if errOwn != nil {
		return errOwn
	}

	var toDelete []int64

	for _, o := range own {
		for _, p := range ml {
			if p.SteamID.Int64() == o.SteamID.Int64() {
				toDelete = append(toDelete, p.SteamID.Int64())
				break
			}
		}
	}

	slog.Info("To delete", slog.Int("count", len(toDelete)))

	httpServer := createHTTPServer(createRouter(database))

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Listener error", slog.String("error", err.Error()))
		}
	}()

	dg, errBot := NewBot(ctx, database, token)
	if errBot != nil {
		return errBot
	}

	slog.Debug("Add bot link", slog.String("link", AddUrl()))

	signal.NotifyContext(ctx, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-ctx.Done()

	if err := dg.Close(); err != nil {
		slog.Error("Failed to properly shutdown discord client", slog.String("error", err.Error()))
	}

	cancelCtx, cancelHTTP := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
	defer cancelHTTP()

	if err := httpServer.Shutdown(cancelCtx); err != nil {
		slog.Error("Failed to cleanly shutdown http service: %s", slog.String("error", err.Error()))
	}

	return nil
}
