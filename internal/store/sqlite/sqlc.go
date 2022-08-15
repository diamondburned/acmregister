package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	_ "modernc.org/sqlite"
)

//go:generate sqlc generate

//go:embed schema.sql
var schemaSQL string

// Versions returns the versions of the schema.
func Versions() []string {
	return strings.Split(schemaSQL, "-- NEW VERSION\n")
}

// Migrate migrates the given database to the latest migrations. It uses the
// user_version pragma.
func Migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return errors.Wrap(err, "cannot begin transaction")
	}
	defer tx.Rollback()

	var v int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return errors.Wrap(err, "cannot get PRAGMA user_version")
	}

	versions := Versions()
	if v >= len(versions) {
		return nil
	}

	for i := v; i < len(versions); i++ {
		_, err := tx.ExecContext(ctx, versions[i])
		if err != nil {
			return errors.Wrapf(err, "cannot apply migration %d (from 0th)", i)
		}
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintln("PRAGMA user_version =", len(versions))); err != nil {
		return errors.Wrap(err, "cannot set PRAGMA user_version")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "cannot commit new migrations")
	}

	return nil
}
