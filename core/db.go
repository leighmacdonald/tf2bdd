package core

import (
	"context"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"strings"
	"tf2bdd/leagues"
	"tf2bdd/steamid"
)

func openDB(ctx context.Context, dbPath string) (*sql.DB, error) {
	const (
		players = `CREATE TABLE IF NOT EXISTS player (
		    steamid BIGINT PRIMARY KEY,
		    attributes TEXT,
		    last_seen BIGINT,
		    last_name TEXT
		);`
		comp = `CREATE TABLE IF NOT EXISTS comp (
		    comp_id INT PRIMARY KEY,
		    steamid BIGINT NOT NULL,
		    league TEXT NOT NULL,
		    division TEXT NOT NULL,
		    division_rank INTEGER NOT NULL,
		    format TEXT NOT NULL,
		    updated_on BIGINT NOT NULL,
		    FOREIGN KEY (steamid) REFERENCES player(steamid) ON DELETE CASCADE ON UPDATE CASCADE
		);`
	)
	db, err := sql.Open("sqlite3", dbPath+"?multiStatements=true")
	if err != nil {
		return nil, errors.Wrapf(err, "Could not open database: %v", err)
	}
	for _, table := range []string{players, comp} {
		stmt, err := db.PrepareContext(ctx, table)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to setup create table stmt: %v", err)
		}
		_, err = stmt.ExecContext(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to create table: %v", err)
		}
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

func loadSeasons(ctx context.Context, db *sql.DB, lh map[steamid.SID64][]leagues.Season) error {
	const q = `SELECT steamid, league, division, division_rank, format FROM comp`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	lSeasons := make(map[steamid.SID64][]leagues.Season)
	for rows.Next() {
		var (
			sid steamid.SID64
			s   leagues.Season
		)
		if err := rows.Scan(&sid, &s.League, &s.Division, &s.DivisionInt, &s.Format); err != nil {
			return err
		}
		if _, exists := lSeasons[sid]; !exists {
			lSeasons[sid] = []leagues.Season{}
		}
		lSeasons[sid] = append(lSeasons[sid], s)
	}
	lh = lSeasons
	return nil
}

func addSeason(ctx context.Context, db *sql.DB, steamID steamid.SID64, seasons []leagues.Season) error {
	const q = `
		INSERT INTO comp (steamid, league, division, division_rank, format, updated_on) 
		VALUES (?,?,?,?,?, strftime('%s','now'))`
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	pc, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	for _, s := range seasons {
		if _, err := pc.ExecContext(ctx, q, steamID, s.League, s.Division, s.DivisionInt, s.Format); err != nil {
			if err := tx.Rollback(); err != nil {
				return errors.Wrapf(err, "Failed to rollback")
			}
			return err
		}
	}
	return nil
}
