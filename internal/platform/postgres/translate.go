package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/platform/apperr"
)

type Constraints map[string]error

func ConstraintOf(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.ConstraintName
	}
	return ""
}

func Translate(err error, constraints Constraints) error {
	if err == nil {
		return nil
	}
	var app *apperr.Error
	var coded apperr.Coded
	if errors.As(err, &app) || errors.As(err, &coded) {
		return err
	}
	if mapped, ok := constraints[ConstraintOf(err)]; ok {
		return mapped
	}
	if apperr.IsUnavailable(err) {
		return apperr.ErrUnavailable
	}
	return apperr.InternalDetail(err.Error())
}
