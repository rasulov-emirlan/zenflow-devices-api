package postgresql

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/profiles"
)

const uniqueViolation = "23505"

func translateProfilesErr(err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == uniqueViolation {
		return profiles.ErrDuplicateName
	}
	return err
}
