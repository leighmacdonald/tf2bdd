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

	db, errOpen := sql.Open("sqlite3", dbPath+"?multiStatements=true")
	if errOpen != nil {
		return nil, errors.Join(errOpen, errors.New("could not open database"))
	}

	for _, table := range []string{players} {
		stmt, errPrepare := db.PrepareContext(ctx, table)
		if errPrepare != nil {
			return nil, errors.Join(errPrepare, errors.New("failed to setup create table stmt"))
		}

		_, errExec := stmt.ExecContext(ctx)
		if errExec != nil {
			return nil, errors.Join(errExec, errors.New("failed to create table"))
		}
	}

	return db, nil
}

func loadPlayers(ctx context.Context, db *sql.DB, players map[steamid.SteamID]Player) error {
	const q = `SELECT steamid, attributes, last_seen, last_name FROM player`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return err
	}

	defer func() {
		if errClose := rows.Close(); errClose != nil {
			slog.Error("Failed to close rows handle", slog.String("error", errClose.Error()))
		}
	}()

	for rows.Next() {
		var (
			p        Player
			attrs    string
			lastSeen int64
			lastName string
		)
		if errScan := rows.Scan(&p.SteamID, &attrs, &lastSeen, &lastName); errScan != nil {
			return errors.Join(errScan, errors.New("error scanning player row"))
		}
		for _, a := range strings.Split(attrs, ",") {
			p.Attributes = append(p.Attributes, Attributes(a))
		}
		p.LastSeen = LastSeen{
			PlayerName: lastName,
			Time:       lastSeen,
		}
		players[p.SteamID] = p
	}

	return nil
}

func addPlayer(ctx context.Context, db *sql.DB, player Player) error {
	const q = `
		INSERT INTO player (steamid, attributes, last_seen, last_name)
		VALUES(?, ?, ?, ?)`
	var attrs []string
	for _, a := range player.Attributes {
		attrs = append(attrs, string(a))
	}

	if _, err := db.ExecContext(ctx, q, player.SteamID, strings.Join(attrs, ","),
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
