package services

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/forge/repositories"
	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
)

type ConnectionView struct {
	Connection forgetypes.Connection
	WebhookURL *string
}

func (s *ConnectionsService) view(ctx context.Context, c forgetypes.Connection) (ConnectionView, error) {
	views, err := s.views(ctx, []forgetypes.Connection{c})
	if err != nil {
		return ConnectionView{}, err
	}
	return views[0], nil
}

func (s *ConnectionsService) views(ctx context.Context, conns []forgetypes.Connection) ([]ConnectionView, error) {
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
	for _, c := range conns {
		if c.Credentials.SecretID != nil {
			if ref, ok := refs[*c.Credentials.SecretID]; ok {
				c.Credentials.Secret = &ref
			}
		}
		out = append(out, s.viewOf(c))
	}
	return out, nil
}

func (s *ConnectionsService) viewOf(c forgetypes.Connection) ConnectionView {
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
	return s.cfg.Web.PublicURL + "/api/v1/forge/hooks/" + id.String()
}

func (s *ConnectionsService) authorizeConnection(ctx context.Context, p accesstypes.Principal, perm catalogtypes.Permission, nodeID, id uuid.UUID) (forgetypes.Connection, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, perm, nodeID); err != nil {
		return forgetypes.Connection{}, err
	}
	c, err := s.connectionRepository.Find(ctx, id)
	if err != nil {
		return forgetypes.Connection{}, apperr.Wrap(err)
	}
	if c == nil || c.NodeID != nodeID {
		return forgetypes.Connection{}, apperr.New(apperr.NotFound)
	}
	return *c, nil
}

func (s *ConnectionsService) secretFor(ctx context.Context, nodeID uuid.UUID, in *catalogtypes.CredentialsInput) (uuid.UUID, error) {
	id, err := catalogtypes.ParseCredentials(in, forgetypes.InvalidCredentials)
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

func (s *ConnectionsService) CreateConnection(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID, in forgetypes.CreateConnection) (ConnectionView, error) {
	sc, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID)
	if err != nil {
		return ConnectionView{}, err
	}
	if sc.Node.Kind == catalogtypes.KindProject {
		return ConnectionView{}, apperr.Wrap(forgetypes.InvalidContainer)
	}
	settings, err := forgetypes.ValidateCreate(in, int(s.cfg.Jobs.ForgeIntervalSecs))
	if err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	secret, err := s.secretFor(ctx, nodeID, in.Credentials)
	if err != nil {
		return ConnectionView{}, err
	}
	id := uuid.Must(uuid.NewV7())
	if err := s.connectionRepository.Insert(ctx, forgetypes.NewConnection{ID: id, NodeID: nodeID, Settings: settings, SecretID: secret}); err != nil {
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

func (s *ConnectionsService) UpdateConnection(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID, in forgetypes.UpdateConnection) (ConnectionView, error) {
	cur, err := s.authorizeConnection(ctx, p, catalogtypes.PermAccess, nodeID, id)
	if err != nil {
		return ConnectionView{}, err
	}
	settings, err := forgetypes.ValidateUpdate(cur, in)
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
		if client, err := s.sync.clientFor(ctx, c); err == nil {
			if err := client.DeleteHook(ctx, *sec.WebhookID); err != nil {
				slog.Warn("forge webhook not removed", "connection", id, "error", err)
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
	client, err := s.sync.clientFor(ctx, c)
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
