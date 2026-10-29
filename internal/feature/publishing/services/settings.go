package services

import (
	"context"
	"errors"
	"log/slog"
	"net/url"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	"svc-registry/internal/feature/publishing/repositories"
	publishingtypes "svc-registry/internal/feature/publishing/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

type PublicEntry struct {
	Kind publishingtypes.EntryKind
	URL  string
}

type Publication struct {
	Settings publishingtypes.Settings
	Own      publishingtypes.NodeSettings
	From     map[string]*publishingtypes.ChainNode
	Entries  []PublicEntry
	CanEdit  bool
}

type SettingsService struct {
	db                    *pgtx.DB
	cfg                   *config.Config
	catalog               *catalogservices.CatalogServices
	publicationRepository *repositories.PublicationRepository
	hosts                 *HostsService
	knowledge             *knowledgeservices.KnowledgeServices
}

func (s *SettingsService) GetPublication(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (Publication, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id); err != nil {
		return Publication{}, err
	}
	return s.publication(ctx, p, id)
}

func (s *SettingsService) publication(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (Publication, error) {
	chain, err := s.publicationRepository.Chain(ctx, id)
	if err != nil {
		return Publication{}, apperr.Wrap(err)
	}
	merged, from := publishingtypes.Merge(chain)
	origin := func(i int) *publishingtypes.ChainNode {
		if i <= 0 {
			return nil
		}
		return &chain[i]
	}
	out := Publication{Settings: merged, Own: chain[0].Own, CanEdit: accesstypes.RequireRight(p, accesstypes.SectionPublication, accesstypes.ActionUpdate) == nil,
		From: map[string]*publishingtypes.ChainNode{"publish": origin(from.Publish), "versions": origin(from.Versions),
			"default_branch": origin(from.DefaultBranch), "current_release": origin(from.CurrentRelease)}, Entries: []PublicEntry{}}
	for _, e := range publishingtypes.Entries(chain) {
		out.Entries = append(out.Entries, PublicEntry{Kind: e.Kind, URL: s.entryURL(e)})
	}
	return out, nil
}

func (s *SettingsService) entryURL(e publishingtypes.Entry) string {
	if e.Kind == publishingtypes.EntryHost {
		return "https://" + e.Host + "/"
	}
	base := ""
	if u, err := url.Parse(s.cfg.Web.PublicURL); err == nil && s.cfg.Web.PublicURL != "" {
		base = u.Scheme + "://" + u.Host
	}
	return base + "/public/docs/" + url.PathEscape(e.Segment)
}

func (s *SettingsService) publicHost() string {
	if u, err := url.Parse(s.cfg.Web.PublicURL); err == nil {
		return u.Hostname()
	}
	return ""
}

func (s *SettingsService) PutPublication(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in publishingtypes.NodeSettings) (Publication, error) {
	sc, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, id)
	if err != nil {
		return Publication{}, err
	}
	if err := accesstypes.RequireRight(p, accesstypes.SectionPublication, accesstypes.ActionUpdate); err != nil {
		return Publication{}, err
	}
	in, err = publishingtypes.Validate(in, string(sc.Node.Kind), s.publicHost())
	if err != nil {
		return Publication{}, apperr.Wrap(err)
	}
	if in.Name != nil {
		taken, err := s.publicationRepository.SlugTaken(ctx, *in.Name)
		if err != nil {
			return Publication{}, apperr.Wrap(err)
		}
		if taken {
			return Publication{}, apperr.Wrap(publishingtypes.ConflictNameTaken)
		}
	}
	was := s.publishes(ctx, id)
	err = repositories.Err(s.db.InTx(ctx, func(ctx context.Context) error {
		if in.IsEmpty() {
			return s.publicationRepository.Delete(ctx, id)
		}
		return s.publicationRepository.Upsert(ctx, id, in)
	}))
	if err != nil {
		return Publication{}, apperr.Wrap(err)
	}
	s.hosts.forget()
	if !was && s.publishes(ctx, id) && s.knowledge != nil {
		if err := s.knowledge.Settings.ReplanSubtree(ctx, id); err != nil {
			slog.Warn("publication saved, documentation collection not queued", "node", id, "error", err)
		}
	}
	return s.publication(ctx, p, id)
}

func (s *SettingsService) Published(ctx context.Context, projectID uuid.UUID) (bool, error) {
	chain, err := s.publicationRepository.Chain(ctx, projectID)
	if errors.Is(err, apperr.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	merged, _ := publishingtypes.Merge(chain)
	return merged.Publish, nil
}

func (s *SettingsService) publishes(ctx context.Context, id uuid.UUID) bool {
	published, err := s.Published(ctx, id)
	return err == nil && published
}
