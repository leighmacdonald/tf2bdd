package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/leighmacdonald/steamid/v4/steamid"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func main() {
	Execute()
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tf2bdd",
	Short: "Backend services for tf2_bot_detector",
	Long:  `tf2bdd provides HTTP and discord services for use with tf2_bot_detector`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		ctx := context.Background()
		app, errApp := NewApp(ctx, "./db.sqlite")
		if errApp != nil {
			return errApp
		}

		ml, errApp := DownloadMasterList()
		if errApp != nil {
			slog.Warn("Failed to download master list from GH", slog.String("error", errApp.Error()))
		}
		app.LoadMasterIDS(ml)

		opts := DefaultHTTPOpts()
		opts.Handler = NewRouter(app)
		srv := NewHTTPServer(opts)
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("Listener error", slog.String("error", err.Error()))
			}
		}()

		dg, errBot := NewBot(app, token)
		if errBot != nil {
			return errBot
		}

		slog.Debug("Add bot link", slog.String("link", AddUrl()))

		Wait(ctx, func(ctx context.Context) error {
			if err := dg.Close(); err != nil {
				slog.Error("Failed to properly shutdown discord client", slog.String("error", err.Error()))
			}
			c, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
			defer cancel()
			if err := srv.Shutdown(c); err != nil {
				slog.Error("Failed to cleanly shutdown http service: %s", slog.String("error", err.Error()))
			}
			return nil
		})

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("error returned", slog.String("err", err.Error()))
		os.Exit(1)
	}
}
