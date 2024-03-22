package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"

	"github.com/leighmacdonald/steamid/v4/steamid"
	_ "github.com/mattn/go-sqlite3"
)

func openDB(ctx context.Context, dbPath string) (*sql.DB, error) {
	const (
		players = `CREATE TABLE IF NOT EXISTS player (
		    steamid BIGINT PRIMARY KEY,
		    attributes TEXT,
		    last_seen BIGINT,
		    last_name TEXT
		);`
	)

	database, errOpen := sql.Open("sqlite3", dbPath+"?multiStatements=true")
	if errOpen != nil {
		return nil, errors.Join(errOpen, errors.New("could not open database"))
	}

	for _, table := range []string{players} {
		stmt, errPrepare := database.PrepareContext(ctx, table)
		if errPrepare != nil {
			return nil, errors.Join(errPrepare, errors.New("failed to setup create table stmt"))
		}

		_, errExec := stmt.ExecContext(ctx)
		if errExec != nil {
			return nil, errors.Join(errExec, errors.New("failed to create table"))
		}
	}

	return database, nil
}

func getPlayer(ctx context.Context, db *sql.DB, steamID steamid.SteamID) (Player, error) {
	const q = `SELECT steamid, attributes, last_seen, last_name FROM player WHERE steamid = ?`
	var (
		player   Player
		sid      int64
		attrs    string
		lastSeen int64
		lastName string
	)
	if errScan := db.QueryRowContext(ctx, q, steamID.Int64()).Scan(&sid, &attrs, &lastSeen, &lastName); errScan != nil {
		return Player{}, errScan
	}
	player.SteamID = steamid.New(sid)
	for _, a := range strings.Split(attrs, ",") {
		player.Attributes = append(player.Attributes, Attributes(a))
	}
	player.LastSeen = LastSeen{
		PlayerName: lastName,
		Time:       lastSeen,
	}

	return player, nil
}

func getPlayers(ctx context.Context, db *sql.DB) ([]Player, error) {
	const q = `SELECT steamid, attributes, last_seen, last_name FROM player`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to load player"))
	}

	defer func() {
		if errClose := rows.Close(); errClose != nil {
			slog.Error("Failed to close rows handle", slog.String("error", errClose.Error()))
		}
	}()

	var players []Player

	for rows.Next() {
		var (
			player   Player
			sid      int64
			attrs    string
			lastSeen int64
			lastName string
		)
		if errScan := rows.Scan(&sid, &attrs, &lastSeen, &lastName); errScan != nil {
			return nil, errors.Join(errScan, errors.New("error scanning player row"))
		}
		player.SteamID = steamid.New(sid)
		for _, a := range strings.Split(attrs, ",") {
			player.Attributes = append(player.Attributes, Attributes(a))
		}
		player.LastSeen = LastSeen{
			PlayerName: lastName,
			Time:       lastSeen,
		}

		players = append(players, player)
	}

	if rows.Err() != nil {
		slog.Error("rows error", slog.String("error", rows.Err().Error()))
	}

	return players, nil
}

func getCount(ctx context.Context, db *sql.DB) (int, error) {
	var total int
	if err := db.QueryRowContext(ctx, "select count(*) from player").Scan(&total); err != nil {
		return -1, err
	}

	return total, nil
}

func addPlayer(ctx context.Context, db *sql.DB, player Player) error {
	const q = `
		INSERT INTO player (steamid, attributes, last_seen, last_name)
		VALUES(?, ?, ?, ?)`
	var attrs []string
	for _, a := range player.Attributes {
		attrs = append(attrs, string(a))
	}

	if _, err := db.ExecContext(ctx, q, player.SteamID.Int64(), strings.Join(attrs, ","),
		player.LastSeen.Time, player.LastSeen.PlayerName); err != nil {
		return err
	}

	return nil
}

func dropPlayer(ctx context.Context, db *sql.DB, steamID steamid.SteamID) error {
	const q = `DELETE FROM player WHERE steamid = ?`
	if _, err := db.ExecContext(ctx, q, steamID.Int64()); err != nil {
		return errors.Join(err, errors.New("failed to drop user"))
	}

	return nil
}
