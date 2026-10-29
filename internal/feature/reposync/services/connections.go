package services

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	pollingtypes "svc-registry/internal/feature/polling/types"
	"svc-registry/internal/feature/reposync/repositories"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
)

type ConnectionView struct {
	Connection            reposynctypes.Connection
	WebhookURL            *string
	EffectiveIntervalSecs int
	Paused                bool
}

func (s *ConnectionsService) view(ctx context.Context, c reposynctypes.Connection) (ConnectionView, error) {
	views, err := s.views(ctx, []reposynctypes.Connection{c})
	if err != nil {
		return ConnectionView{}, err
	}
	return views[0], nil
}

func (s *ConnectionsService) views(ctx context.Context, conns []reposynctypes.Connection) ([]ConnectionView, error) {
	var ids []uuid.UUID
	for _, c := range conns {
		if c.Credentials.SecretID != nil {
			ids = append(ids, *c.Credentials.SecretID)
		}
	}
	refs, err := s.catalog.SecretResolver.SecretRefs(ctx, ids)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make([]ConnectionView, 0, len(conns))
	effective := map[uuid.UUID]pollingtypes.Effective{}
	for _, c := range conns {
		e, ok := effective[c.NodeID]
		if !ok {
			e = s.sync.effective(ctx, c.NodeID)
			effective[c.NodeID] = e
		}
		if c.Credentials.SecretID != nil {
			if ref, ok := refs[*c.Credentials.SecretID]; ok {
				c.Credentials.Secret = &ref
			}
		}
		v := s.viewOf(c)
		v.EffectiveIntervalSecs, v.Paused = s.sync.intervalSecs(c, e), e.Off()
		out = append(out, v)
	}
	return out, nil
}

func (s *ConnectionsService) viewOf(c reposynctypes.Connection) ConnectionView {
	v := ConnectionView{Connection: c}
	if c.WebhookMode != nil {
		if u := s.webhookURL(c.ID); u != "" {
			v.WebhookURL = &u
		}
	}
	return v
}

func (s *ConnectionsService) webhookURL(id uuid.UUID) string {
	if s.cfg.Web.PublicURL == "" {
		return ""
	}
	return s.cfg.Web.PublicURL + "/api/v1/repo-sync/hooks/" + id.String()
}

func (s *ConnectionsService) authorizeConnection(ctx context.Context, p accesstypes.Principal, perm catalogtypes.Permission, nodeID, id uuid.UUID) (reposynctypes.Connection, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, perm, nodeID); err != nil {
		return reposynctypes.Connection{}, err
	}
	c, err := s.connectionRepository.Find(ctx, id)
	if err != nil {
		return reposynctypes.Connection{}, apperr.Wrap(err)
	}
	if c == nil || c.NodeID != nodeID {
		return reposynctypes.Connection{}, apperr.New(apperr.NotFound)
	}
	return *c, nil
}

func (s *ConnectionsService) secretFor(ctx context.Context, nodeID uuid.UUID, in *catalogtypes.CredentialsInput) (uuid.UUID, error) {
	id, err := catalogtypes.ParseCredentials(in, reposynctypes.InvalidCredentials)
	if err != nil {
		return uuid.Nil, apperr.Wrap(err)
	}
	if _, err := s.catalog.SecretResolver.SecretFor(ctx, &nodeID, id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *ConnectionsService) ListConnections(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID) ([]ConnectionView, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermRead, nodeID); err != nil {
		return nil, err
	}
	conns, err := s.connectionRepository.List(ctx, nodeID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.views(ctx, conns)
}

func (s *ConnectionsService) CreateConnection(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID, in reposynctypes.CreateConnection) (ConnectionView, error) {
	sc, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID)
	if err != nil {
		return ConnectionView{}, err
	}
	if sc.Node.Kind == catalogtypes.KindProject {
		return ConnectionView{}, apperr.Wrap(reposynctypes.InvalidContainer)
	}
	settings, err := reposynctypes.ValidateCreate(in)
	if err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	secret, err := s.secretFor(ctx, nodeID, in.Credentials)
	if err != nil {
		return ConnectionView{}, err
	}
	id := uuid.Must(uuid.NewV7())
	if err := s.connectionRepository.Insert(ctx, reposynctypes.NewConnection{ID: id, NodeID: nodeID, Settings: settings, SecretID: secret}); err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	return s.reload(ctx, id)
}

func (s *ConnectionsService) GetConnection(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) (ConnectionView, error) {
	c, err := s.authorizeConnection(ctx, p, catalogtypes.PermRead, nodeID, id)
	if err != nil {
		return ConnectionView{}, err
	}
	return s.view(ctx, c)
}

func (s *ConnectionsService) UpdateConnection(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID, in reposynctypes.UpdateConnection) (ConnectionView, error) {
	cur, err := s.authorizeConnection(ctx, p, catalogtypes.PermAccess, nodeID, id)
	if err != nil {
		return ConnectionView{}, err
	}
	settings, err := reposynctypes.ValidateUpdate(cur, in)
	if err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	var secret *uuid.UUID
	if in.Credentials != nil {
		sid, err := s.secretFor(ctx, nodeID, in.Credentials)
		if err != nil {
			return ConnectionView{}, err
		}
		secret = &sid
	}
	if err := s.connectionRepository.Update(ctx, id, settings, secret); err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	return s.reload(ctx, id)
}

func (s *ConnectionsService) reload(ctx context.Context, id uuid.UUID) (ConnectionView, error) {
	c, err := s.connectionRepository.Find(ctx, id)
	if err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	if c == nil {
		return ConnectionView{}, apperr.Wrap(apperr.ErrNotFound)
	}
	return s.view(ctx, *c)
}

func (s *ConnectionsService) DeleteConnection(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) error {
	c, err := s.authorizeConnection(ctx, p, catalogtypes.PermAccess, nodeID, id)
	if err != nil {
		return err
	}
	if sec, err := s.connectionRepository.Secrets(ctx, id); err == nil && sec != nil && sec.WebhookID != nil {
		if client, err := s.sync.clientFor(ctx, c, s.sync.effective(ctx, c.NodeID).Timeout()); err == nil {
			if err := client.DeleteHook(ctx, *sec.WebhookID); err != nil {
				slog.Warn("hosting webhook not removed", "connection", id, "error", err)
			}
		}
	}
	return apperr.Wrap(s.connectionRepository.Delete(ctx, id))
}

func (s *ConnectionsService) CheckConnection(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) error {
	c, err := s.authorizeConnection(ctx, p, catalogtypes.PermWrite, nodeID, id)
	if err != nil {
		return err
	}
	client, err := s.sync.clientFor(ctx, c, s.sync.effective(ctx, c.NodeID).Timeout())
	if err != nil {
		return apperr.Wrap(err)
	}
	return apperr.Wrap(client.Check(ctx))
}

type ConnectionsService struct {
	catalog              *catalogservices.CatalogServices
	cfg                  *config.Config
	connectionRepository *repositories.ConnectionRepository
	sync                 *SyncService
}
