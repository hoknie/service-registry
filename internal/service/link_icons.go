package service

import (
	"context"
	"io"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/links"
)

func UploadLinkIcon(ctx context.Context, state *State, p Principal, nodeID uuid.UUID, data []byte) (string, error) {
	if _, err := Authorize(ctx, state, p, catalog.PermWrite, nodeID); err != nil {
		return "", err
	}
	if state.Icons == nil {
		return "", apperr.Wrap(links.ConflictUploadsDisabled)
	}
	id, err := state.Icons.Put(data)
	if err != nil {
		return "", apperr.Wrap(err)
	}
	return id, nil
}

func OpenLinkIcon(state *State, id string) (io.ReadCloser, int64, error) {
	if state.Icons == nil {
		return nil, 0, apperr.New(apperr.NotFound)
	}
	f, size, err := state.Icons.Open(id)
	if err != nil {
		return nil, 0, apperr.Wrap(err)
	}
	return f, size, nil
}
