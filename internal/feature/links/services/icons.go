package services

import (
	"context"
	"io"
	"slices"
	"time"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	linksports "svc-registry/internal/feature/links/ports"
	"svc-registry/internal/feature/links/repositories"
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

func (s *IconsService) PruneLinkIcons(ctx context.Context) (int, error) {
	if s.icons == nil {
		return 0, nil
	}
	files, err := s.icons.List()
	if err != nil {
		return 0, apperr.Wrap(err)
	}
	cutoff := time.Now().Add(-s.grace)
	var stale []string
	var ids []string
	for _, f := range files {
		if !f.Modified.Before(cutoff) {
			continue
		}
		stale = append(stale, f.Name)
		if !f.Upload {
			ids = append(ids, f.Name)
		}
	}
	if len(stale) == 0 {
		return 0, nil
	}
	used, err := s.templateRepository.ReferencedIcons(ctx, ids)
	if err != nil {
		return 0, apperr.Wrap(err)
	}
	deleted := 0
	for _, name := range stale {
		if slices.Contains(used, name) {
			continue
		}
		if err := s.icons.Remove(name); err != nil {
			return deleted, apperr.Wrap(err)
		}
		deleted++
	}
	return deleted, nil
}

type IconsService struct {
	catalog            *catalogservices.CatalogServices
	icons              linksports.IconFilesPort
	templateRepository *repositories.TemplateRepository
	grace              time.Duration
}
