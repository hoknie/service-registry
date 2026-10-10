package service

import (
	"context"
	"io"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/links"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) UploadLinkIcon(ctx context.Context, p access.Principal, nodeID uuid.UUID, data []byte) (string, error) {
	if _, err := s.catalog.Authorize(ctx, p, catalog.PermWrite, nodeID); err != nil {
		return "", err
	}
	if s.icons == nil {
		return "", apperr.Wrap(links.ConflictUploadsDisabled)
	}
	id, err := s.icons.Put(data)
	if err != nil {
		return "", apperr.Wrap(err)
	}
	return id, nil
}

func (s *Service) OpenLinkIcon(id string) (io.ReadCloser, int64, string, error) {
	if s.icons == nil {
		return nil, 0, "", apperr.New(apperr.NotFound)
	}
	f, size, err := s.icons.Open(id)
	if err != nil {
		return nil, 0, "", apperr.Wrap(err)
	}
	return f, size, s.icons.ContentType(id), nil
}
