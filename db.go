package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/leighmacdonald/steamid/v4/steamid"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func openDB(dbPath string) (*sql.DB, error) {
	database, errOpen := sql.Open("sqlite3", dbPath)
	if errOpen != nil {
		return nil, errors.Join(errOpen, errors.New("could not open database"))
	}

	return database, nil
}

func setupDB(ctx context.Context, database *sql.DB) error {
	const query = `
		CREATE TABLE IF NOT EXISTS player (
		    steamid BIGINT PRIMARY KEY,
		    attributes TEXT,
		    last_seen BIGINT,
		    last_name TEXT,
		    author BIGINT default 0,
		    created_on integer default 0
		);`

	stmt, errPrepare := database.PrepareContext(ctx, query)
	if errPrepare != nil {
		return errors.Join(errPrepare, errors.New("failed to setup create table stmt"))
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			slog.Error("Error closing prepared statement", slog.String("error", err.Error()))
		}
	}()

	_, errExec := stmt.ExecContext(ctx)
	if errExec != nil {
		return errors.Join(errExec, errors.New("failed to create table"))
	}

	return nil
}

func getPlayer(ctx context.Context, database *sql.DB, steamID steamid.SteamID) (Player, error) {
	const query = `SELECT steamid, attributes, last_seen, last_name, author, created_on FROM player WHERE steamid = ?`

	var (
		player    Player
		sid       int64
		attrs     string
		lastSeen  int64
		lastName  string
		createdOn int64
	)

	if errScan := database.
		QueryRowContext(ctx, query, steamID.Int64()).
		Scan(&sid, &attrs, &lastSeen, &lastName, &player.Author, &createdOn); errScan != nil {
		return Player{}, errScan
	}

	player.CreatedOn = time.Unix(createdOn, 0)
	player.SteamID = steamid.New(sid)
	player.Attributes = strings.Split(strings.ToLower(attrs), ",")
	player.LastSeen = LastSeen{
		PlayerName: lastName,
		Time:       lastSeen,
	}

	return player, nil
}

func getPlayers(ctx context.Context, db *sql.DB) ([]Player, error) {
	const query = `SELECT steamid, attributes, last_seen, last_name, author, created_on FROM player`

	rows, err := db.QueryContext(ctx, query)
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
			player    Player
			sid       int64
			attrs     string
			lastSeen  int64
			lastName  string
			createdOn int64
		)

		if errScan := rows.Scan(&sid, &attrs, &lastSeen, &lastName, &player.Author, &createdOn); errScan != nil {
			return nil, errors.Join(errScan, errors.New("error scanning player row"))
		}

		player.CreatedOn = time.Unix(createdOn, 0)
		player.SteamID = steamid.New(sid)
		player.Attributes = strings.Split(strings.ToLower(attrs), ",")
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

func addPlayer(ctx context.Context, db *sql.DB, player Player, author int64) error {
	const query = `
		INSERT INTO player (steamid, attributes, last_seen, last_name, author, created_on)
		VALUES(?, ?, ?, ?, ?, ?)`

	if _, err := db.ExecContext(ctx, query,
		player.SteamID.Int64(),
		strings.ToLower(strings.Join(player.Attributes, ",")),
		player.LastSeen.Time,
		player.LastSeen.PlayerName,
		author,
		int(time.Now().Unix())); err != nil {
		return err
	}

	return nil
}

func dropPlayer(ctx context.Context, db *sql.DB, steamID steamid.SteamID) error {
	const query = `DELETE FROM player WHERE steamid = ?`

	if _, err := db.ExecContext(ctx, query, steamID.Int64()); err != nil {
		return errors.Join(err, errors.New("failed to drop user"))
	}

	return nil
}
