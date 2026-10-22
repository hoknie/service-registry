package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/platform/apperr"
)

type Constraints map[string]error

func ConstraintOf(err error) string {
	if pg, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pg.ConstraintName
	}
	return ""
}

func Translate(err error, constraints Constraints) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*apperr.Error](err); ok {
		return err
	}
	if _, ok := errors.AsType[apperr.Coded](err); ok {
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
