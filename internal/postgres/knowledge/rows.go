package knowledge

import (
	"errors"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	domain "svc-registry/internal/knowledge"
)

func dbErr(err error) error {
	if err == nil {
		return nil
	}
	var invalid domain.Invalid
	var conflict domain.Conflict
	var internalErr *domain.InternalError
	if errors.As(err, &invalid) || errors.As(err, &conflict) || errors.Is(err, domain.ErrNotFound) || errors.As(err, &internalErr) {
		return err
	}
	if apperr.IsUnavailable(err) {
		return domain.ErrUnavailable
	}
	return &domain.InternalError{Detail: err.Error()}
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

func (r snapshotRow) snapshot() domain.Snapshot {
	return domain.Snapshot{ID: r.ID, ProjectID: r.ProjectID, Branch: r.Branch, Commit: r.CommitSHA, Status: domain.Status(r.Status),
		ErrorCode: r.ErrorCode, Files: int(r.Files), Bytes: r.Bytes, Skipped: int(r.Skipped), Truncated: r.Truncated, CollectedAt: r.CollectedAt}
}
