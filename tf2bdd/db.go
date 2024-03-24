package tf2bdd

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/leighmacdonald/steamid/v4/steamid"
	"github.com/ncruces/go-sqlite3"
)

//go:embed migrations/*.sql
var migrations embed.FS

var (
	ErrMigration        = errors.New("could not migrate db schema")
	ErrStoreIOFSOpen    = errors.New("failed to create migration iofs")
	ErrStoreIOFSClose   = errors.New("failed to close migration iofs")
	ErrStoreDriver      = errors.New("failed to create db driver")
	ErrCreateMigration  = errors.New("failed to create migrator")
	ErrPerformMigration = errors.New("failed to migrate database")
	ErrDuplicate        = errors.New("duplicate entry")
	ErrNotFound         = errors.New("entry not found")
)

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	var sqliteErr *sqlite3.Error
	if errors.As(err, &sqliteErr) {
		switch {
		case errors.Is(sqliteErr.Code(), sqlite3.CONSTRAINT):
			return ErrDuplicate
		default:
			return fmt.Errorf("unhandled sqlite error: %w", err)
		}
	}

	return err
}

func OpenDB(dbPath string) (*sql.DB, error) {
	database, errOpen := sql.Open("sqlite3", dbPath)
	if errOpen != nil {
		return nil, errors.Join(errOpen, errors.New("could not open database"))
	}

	return database, nil
}

func SetupDB(database *sql.DB) error {
	slog.Info("Performing database migration")
	if errMigrate := migrateDB(database); errMigrate != nil {
		return errors.Join(errMigrate, ErrMigration)
	}

	return nil
}

func migrateDB(database *sql.DB) error {
	fsDriver, errIofs := iofs.New(migrations, "migrations")
	if errIofs != nil {
		return errors.Join(errIofs, ErrStoreIOFSOpen)
	}

	sqlDriver, errDriver := sqlite.WithInstance(database, &sqlite.Config{})
	if errDriver != nil {
		return errors.Join(errDriver, ErrStoreDriver)
	}

	migrator, errNewMigrator := migrate.NewWithInstance("iofs", fsDriver, "sqlite", sqlDriver)
	if errNewMigrator != nil {
		return errors.Join(errNewMigrator, ErrCreateMigration)
	}

	if errMigrate := migrator.Up(); errMigrate != nil && !errors.Is(errMigrate, migrate.ErrNoChange) {
		return errors.Join(errMigrate, ErrPerformMigration)
	}

	// We do not call migrator.Close and instead close the fsDriver manually.
	// This is because sqlite will wipe the db when :memory: is used and the connection closes
	// for any reason, which the migrator does when called.
	if errClose := fsDriver.Close(); errClose != nil {
		return errors.Join(errClose, ErrStoreIOFSClose)
	}

	return nil
}

const proofSep = "^^"

type Proof []string

func (p *Proof) Scan(value interface{}) error {
	strVal, ok := value.(string)
	if !ok {
		return errors.New("invalid type")
	}
	if strVal == "" {
		*p = []string{}

		return nil
	}

	*p = strings.Split(strVal, proofSep)

	return nil
}

func (p Proof) Value() (driver.Value, error) {
	return strings.Join(p, proofSep), nil
}

func updatePlayer(ctx context.Context, database *sql.DB, player Player) error {
	const query = `
		UPDATE player 
		SET attributes = ?,
		    last_seen = ?,
		    last_name = ?,
		    author = ?,
		    proof = ?
		WHERE steamid = ?`

	if _, errExec := database.ExecContext(ctx, query, strings.Join(player.Attributes, ","), player.LastSeen.Time, player.LastSeen.PlayerName,
		player.Author, player.Proof, player.SteamID.Int64()); errExec != nil {
		return errExec
	}

	return nil
}

func getPlayer(ctx context.Context, database *sql.DB, steamID steamid.SteamID) (Player, error) {
	const query = `SELECT steamid, attributes, last_seen, last_name, author, created_on, proof FROM player WHERE steamid = ?`

	var (
		player    Player
		sid       int64
		attrs     string
		lastSeen  int64
		lastName  string
		createdOn int64
		proof     Proof
	)

	if errScan := database.
		QueryRowContext(ctx, query, steamID.Int64()).
		Scan(&sid, &attrs, &lastSeen, &lastName, &player.Author, &createdOn, &proof); errScan != nil {
		return Player{}, dbErr(errScan)
	}

	player.CreatedOn = time.Unix(createdOn, 0)
	player.SteamID = steamid.New(sid)
	player.Attributes = strings.Split(strings.ToLower(attrs), ",")
	player.LastSeen = LastSeen{
		PlayerName: lastName,
		Time:       lastSeen,
	}
	player.Proof = proof

	return player, nil
}

func getPlayers(ctx context.Context, db *sql.DB) ([]Player, error) {
	const query = `SELECT steamid, attributes, last_seen, last_name, author, created_on, proof FROM player`

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
			proof     Proof
		)

		if errScan := rows.Scan(&sid, &attrs, &lastSeen, &lastName, &player.Author, &createdOn, &proof); errScan != nil {
			return nil, errors.Join(errScan, errors.New("error scanning player row"))
		}

		player.CreatedOn = time.Unix(createdOn, 0)
		player.SteamID = steamid.New(sid)
		player.Attributes = strings.Split(strings.ToLower(attrs), ",")
		player.LastSeen = LastSeen{
			PlayerName: lastName,
			Time:       lastSeen,
		}
		player.Proof = proof

		players = append(players, player)
	}

	if rows.Err() != nil {
		slog.Error("rows error", slog.String("error", rows.Err().Error()))
	}

	return players, nil
}

func AddPlayer(ctx context.Context, db *sql.DB, player Player, author int64) error {
	const query = `
		INSERT INTO player (steamid, attributes, last_seen, last_name, author, created_on, proof)
		VALUES(?, ?, ?, ?, ?, ?, ?)`

	if _, err := db.ExecContext(ctx, query,
		player.SteamID.Int64(),
		strings.ToLower(strings.Join(player.Attributes, ",")),
		player.LastSeen.Time,
		player.LastSeen.PlayerName,
		author,
		int(time.Now().Unix()),
		player.Proof); err != nil {
		return dbErr(err)
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
