package repository

import (
	"errors"

	"github.com/google/uuid"

	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/apperr"
)

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	var invalid knowledge.Invalid
	var conflict knowledge.Conflict
	var internalErr *knowledge.InternalError
	if errors.As(err, &invalid) || errors.As(err, &conflict) || errors.Is(err, knowledge.ErrNotFound) || errors.As(err, &internalErr) {
		return err
	}
	if apperr.IsUnavailable(err) {
		return knowledge.ErrUnavailable
	}
	return &knowledge.InternalError{Detail: err.Error()}
}

type snapshotRow struct {
	ID          uuid.UUID `db:"id"`
	ProjectID   uuid.UUID `db:"project_id"`
	Branch      string    `db:"branch"`
	CommitSHA   string    `db:"commit_sha"`
	Status      string    `db:"status"`
	ErrorCode   *string   `db:"error_code"`
	Files       int32     `db:"files"`
	Bytes       int64     `db:"bytes"`
	Skipped     int32     `db:"skipped"`
	Truncated   bool      `db:"truncated"`
	CollectedAt string    `db:"collected_at"`
}

func (r snapshotRow) snapshot() knowledge.Snapshot {
	return knowledge.Snapshot{ID: r.ID, ProjectID: r.ProjectID, Branch: r.Branch, Commit: r.CommitSHA, Status: knowledge.Status(r.Status),
		ErrorCode: r.ErrorCode, Files: int(r.Files), Bytes: r.Bytes, Skipped: int(r.Skipped), Truncated: r.Truncated, CollectedAt: r.CollectedAt}
}

type signalRow struct {
	ProjectID uuid.UUID `db:"project_id"`
	Kind      string    `db:"kind"`
	Running   bool      `db:"running"`
	Queued    bool      `db:"queued"`
	Failed    bool      `db:"failed"`
	Code      *string   `db:"code"`
	LastAt    *string   `db:"last_at"`
	Pending   *int64    `db:"pending"`
}

func Err(err error) error { return dbErr(err) }
