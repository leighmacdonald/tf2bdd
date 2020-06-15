package core

import (
	"context"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"strings"
	"tf2bdd/steamid"
	"time"
)

func openDB(ctx context.Context, dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not open database: %v", err)
	}
	c, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	stmt, err := db.PrepareContext(c, `
		CREATE TABLE IF NOT EXISTS player (
		    steamid BIGINT PRIMARY KEY,
		    attributes TEXT,
		    last_seen BIGINT,
		    last_name TEXT
		)`)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to setup create table stmt: %v", err)
	}
	_, err = stmt.ExecContext(c)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create table: %v", err)
	}
	return db, nil
}

func loadPlayers(ctx context.Context, db *sql.DB, players map[steamid.SID64]Player) error {
	const q = `SELECT steamid, attributes, last_seen, last_name FROM player`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Errorf("Failed to close rows handle: %v", err)
		}
	}()
	for rows.Next() {
		var (
			p        Player
			attrs    string
			lastSeen int64
			lastName string
		)
		if err := rows.Scan(&p.SteamID, &attrs, &lastSeen, &lastName); err != nil {
			return errors.Wrapf(err, "Error scanning player row: %v", err)
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

func dropPlayer(ctx context.Context, db *sql.DB, steamID steamid.SID64) error {
	const q = `DELETE FROM player WHERE steamid = ?`
	if _, err := db.ExecContext(ctx, q, steamID.Int64()); err != nil {
		return errors.Wrapf(err, "Failed to drop user: %v", err)
	}
	return nil
}
