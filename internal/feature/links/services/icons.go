package services

import (
	"context"
	"io"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	linksports "svc-registry/internal/feature/links/ports"
	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/apperr"
)

func (s *IconsService) UploadLinkIcon(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID, data []byte) (string, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, nodeID); err != nil {
		return "", err
	}
	if s.icons == nil {
		return "", apperr.Wrap(linkstypes.ConflictUploadsDisabled)
	}
	id, err := s.icons.Put(data)
	if err != nil {
		return "", apperr.Wrap(err)
	}
	return id, nil
}

func (s *IconsService) OpenLinkIcon(id string) (io.ReadCloser, int64, string, error) {
	if s.icons == nil {
		return nil, 0, "", apperr.New(apperr.NotFound)
	}
	f, size, err := s.icons.Open(id)
	if err != nil {
		return nil, 0, "", apperr.Wrap(err)
	}
	return f, size, s.icons.ContentType(id), nil
}

type IconsService struct {
	catalog *catalogservices.CatalogServices
	icons   linksports.IconFilesPort
}
