package postgresql

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/domains/deviceprofiles"
)

const uniqueViolation = "23505"

func translateDeviceProfilesErr(err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == uniqueViolation {
		return deviceprofiles.ErrDuplicateName
	}
	return err
}
