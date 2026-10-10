package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/forge"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/secretbox"
)

type ConnectionView struct {
	Connection forge.Connection
	WebhookURL *string
}

type WebhookSetup struct {
	Mode   forge.WebhookMode
	URL    string
	Secret *string
}

type PreviewItem struct {
	FullPath string
	Action   string
	Code     *string
}

func (s *Service) view(c forge.Connection) ConnectionView {
	v := ConnectionView{Connection: c}
	if c.WebhookMode != nil {
		if u := s.webhookURL(c.ID); u != "" {
			v.WebhookURL = &u
		}
	}
	return v
}

func (s *Service) webhookURL(id uuid.UUID) string {
	if s.cfg.Web.PublicURL == "" {
		return ""
	}
	return s.cfg.Web.PublicURL + "/api/v1/forge/hooks/" + id.String()
}

func (s *Service) authorizeConnection(ctx context.Context, p access.Principal, perm catalog.Permission, nodeID, id uuid.UUID) (forge.Connection, error) {
	if _, err := s.catalog.Authorize(ctx, p, perm, nodeID); err != nil {
		return forge.Connection{}, err
	}
	c, err := s.connections.Find(ctx, id)
	if err != nil {
		return forge.Connection{}, apperr.Wrap(err)
	}
	if c == nil || c.NodeID != nodeID {
		return forge.Connection{}, apperr.New(apperr.NotFound)
	}
	return *c, nil
}

func (s *Service) storeCredentials(id uuid.UUID, c forge.ValidCredentials) (forge.StoredCredentials, error) {
	if c.Ref != nil {
		return forge.StoredCredentials{Ref: c.Ref}, nil
	}
	if !s.secrets.CanEncrypt() {
		return forge.StoredCredentials{}, apperr.Wrap(forge.ConflictSecretsKeyMissing)
	}
	enc, err := s.secrets.Seal(*c.Token, secretbox.AAD(forge.Table, id.String(), forge.ColumnCredentials))
	if err != nil {
		return forge.StoredCredentials{}, apperr.Internalf("encrypt: %v", err)
	}
	fp := secretbox.Fingerprint(*c.Token)
	return forge.StoredCredentials{Enc: &enc, Fingerprint: &fp}, nil
}

func (s *Service) ListConnections(ctx context.Context, p access.Principal, nodeID uuid.UUID) ([]ConnectionView, error) {
	if _, err := s.catalog.Authorize(ctx, p, catalog.PermRead, nodeID); err != nil {
		return nil, err
	}
	conns, err := s.connections.List(ctx, nodeID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make([]ConnectionView, 0, len(conns))
	for _, c := range conns {
		out = append(out, s.view(c))
	}
	return out, nil
}

func (s *Service) CreateConnection(ctx context.Context, p access.Principal, nodeID uuid.UUID, in forge.CreateConnection) (ConnectionView, error) {
	sc, err := s.catalog.Authorize(ctx, p, catalog.PermAccess, nodeID)
	if err != nil {
		return ConnectionView{}, err
	}
	if sc.Node.Kind == catalog.KindProject {
		return ConnectionView{}, apperr.Wrap(forge.InvalidContainer)
	}
	settings, err := forge.ValidateCreate(in, int(s.cfg.Jobs.ForgeIntervalSecs))
	if err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	creds, err := forge.ValidateCredentials(in.Credentials)
	if err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	id := uuid.Must(uuid.NewV7())
	stored, err := s.storeCredentials(id, creds)
	if err != nil {
		return ConnectionView{}, err
	}
	c, err := s.connections.Insert(ctx, forge.NewConnection{ID: id, NodeID: nodeID, Settings: settings, Credentials: stored})
	if err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	return s.view(c), nil
}

func (s *Service) GetConnection(ctx context.Context, p access.Principal, nodeID, id uuid.UUID) (ConnectionView, error) {
	c, err := s.authorizeConnection(ctx, p, catalog.PermRead, nodeID, id)
	if err != nil {
		return ConnectionView{}, err
	}
	return s.view(c), nil
}

func (s *Service) UpdateConnection(ctx context.Context, p access.Principal, nodeID, id uuid.UUID, in forge.UpdateConnection) (ConnectionView, error) {
	cur, err := s.authorizeConnection(ctx, p, catalog.PermAccess, nodeID, id)
	if err != nil {
		return ConnectionView{}, err
	}
	settings, err := forge.ValidateUpdate(cur, in)
	if err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	var stored *forge.StoredCredentials
	if in.Credentials != nil {
		creds, err := forge.ValidateCredentials(in.Credentials)
		if err != nil {
			return ConnectionView{}, apperr.Wrap(err)
		}
		sc, err := s.storeCredentials(id, creds)
		if err != nil {
			return ConnectionView{}, err
		}
		stored = &sc
	}
	c, err := s.connections.Update(ctx, id, settings, stored)
	if err != nil {
		return ConnectionView{}, apperr.Wrap(err)
	}
	return s.view(c), nil
}

func (s *Service) DeleteConnection(ctx context.Context, p access.Principal, nodeID, id uuid.UUID) error {
	c, err := s.authorizeConnection(ctx, p, catalog.PermAccess, nodeID, id)
	if err != nil {
		return err
	}
	if sec, err := s.connections.Secrets(ctx, id); err == nil && sec != nil && sec.WebhookID != nil {
		if client, err := s.clientFor(ctx, c); err == nil {
			if err := client.DeleteHook(ctx, *sec.WebhookID); err != nil {
				slog.Warn("forge webhook not removed", "connection", id, "error", err)
			}
		}
	}
	return apperr.Wrap(s.connections.Delete(ctx, id))
}

func (s *Service) CheckConnection(ctx context.Context, p access.Principal, nodeID, id uuid.UUID) error {
	c, err := s.authorizeConnection(ctx, p, catalog.PermWrite, nodeID, id)
	if err != nil {
		return err
	}
	client, err := s.clientFor(ctx, c)
	if err != nil {
		return apperr.Wrap(err)
	}
	return apperr.Wrap(client.Check(ctx))
}

func (s *Service) PreviewConnection(ctx context.Context, p access.Principal, nodeID, id uuid.UUID) ([]PreviewItem, error) {
	c, err := s.authorizeConnection(ctx, p, catalog.PermWrite, nodeID, id)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(ctx, c)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	targets, orphans, err := s.remoteState(ctx, c, client)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	groups, err := s.repositories.Groups(ctx, c.ID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	items := make([]PreviewItem, 0, len(targets)+len(orphans))
	for _, t := range targets {
		item, err := s.previewOne(ctx, c, groups, t)
		if err != nil {
			return nil, apperr.Wrap(err)
		}
		items = append(items, item)
	}
	for _, o := range orphans {
		items = append(items, PreviewItem{FullPath: o.FullPath, Action: "orphan"})
	}
	return items, nil
}

func (s *Service) previewOne(ctx context.Context, c forge.Connection, groups map[string]uuid.UUID, t forge.Target) (PreviewItem, error) {
	r := t.Remote
	item := PreviewItem{FullPath: r.FullPath}
	parent := &c.NodeID
	segs := forge.Subgroups(c.OwnerPath, r.FullPath, c.MirrorSubgroups)
	for i := range segs {
		id, ok := groups[strings.ToLower(forge.GroupPath(c.OwnerPath, segs, i+1))]
		if !ok {
			clash, err := s.catalog.FindChild(ctx, parent, forge.Slugify(segs[i]))
			if err != nil {
				return item, err
			}
			if clash != nil {
				return skipped(item, catalog.ConflictSlugTaken.Code()), nil
			}
			parent = nil
			break
		}
		parent = &id
	}
	slug := forge.Slugify(r.Name)
	if t.Link == nil {
		item.Action = "create"
		if parent != nil {
			clash, err := s.catalog.FindChild(ctx, parent, slug)
			if err != nil {
				return item, err
			}
			if clash != nil {
				return skipped(item, catalog.ConflictSlugTaken.Code()), nil
			}
		}
		return item, nil
	}
	node, err := s.catalog.FindNode(ctx, t.Link.ProjectID)
	if err != nil {
		return item, err
	}
	item.Action = "update"
	if node != nil && (parent == nil || node.ParentID == nil || *node.ParentID != *parent ||
		(!strings.EqualFold(t.Link.FullPath, r.FullPath) && node.Slug != slug)) {
		item.Action = "move"
	}
	return item, nil
}

func skipped(item PreviewItem, code string) PreviewItem {
	item.Action = "skip"
	item.Code = &code
	return item
}

func (s *Service) StartSync(ctx context.Context, p access.Principal, nodeID, id uuid.UUID) error {
	if _, err := s.authorizeConnection(ctx, p, catalog.PermWrite, nodeID, id); err != nil {
		return err
	}
	return apperr.Wrap(s.connections.ScheduleNow(ctx, id, forge.TriggerManual))
}

func (s *Service) ListRuns(ctx context.Context, p access.Principal, nodeID, id uuid.UUID) ([]forge.Run, error) {
	if _, err := s.authorizeConnection(ctx, p, catalog.PermRead, nodeID, id); err != nil {
		return nil, err
	}
	runs, err := s.runs.List(ctx, id, int(s.cfg.Jobs.ForgeRunsKept))
	return runs, apperr.Wrap(err)
}

func (s *Service) SetupWebhook(ctx context.Context, p access.Principal, nodeID, id uuid.UUID, rawMode string) (WebhookSetup, error) {
	c, err := s.authorizeConnection(ctx, p, catalog.PermAccess, nodeID, id)
	if err != nil {
		return WebhookSetup{}, err
	}
	mode, err := forge.ValidateWebhookMode(rawMode)
	if err != nil {
		return WebhookSetup{}, apperr.Wrap(err)
	}
	url := s.webhookURL(id)
	if url == "" {
		return WebhookSetup{}, apperr.Wrap(forge.ConflictPublicURLMissing)
	}
	if !s.secrets.CanEncrypt() {
		return WebhookSetup{}, apperr.Wrap(forge.ConflictSecretsKeyMissing)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return WebhookSetup{}, apperr.Internalf("no randomness: %v", err)
	}
	secret := hex.EncodeToString(raw)
	enc, err := s.secrets.Seal(secret, secretbox.AAD(forge.Table, id.String(), forge.ColumnWebhookSecret))
	if err != nil {
		return WebhookSetup{}, apperr.Internalf("encrypt: %v", err)
	}
	prev, err := s.connections.Secrets(ctx, id)
	if err != nil {
		return WebhookSetup{}, apperr.Wrap(err)
	}
	var client forge.Client
	needClient := mode == forge.WebhookRegister || (prev != nil && prev.WebhookID != nil)
	if needClient {
		if client, err = s.clientFor(ctx, c); err != nil {
			return WebhookSetup{}, apperr.Wrap(err)
		}
	}
	var hookID *string
	if mode == forge.WebhookRegister {
		created, err := client.RegisterHook(ctx, url, secret)
		if err != nil {
			return WebhookSetup{}, apperr.Wrap(err)
		}
		hookID = &created
	}
	if prev != nil && prev.WebhookID != nil {
		if err := client.DeleteHook(ctx, *prev.WebhookID); err != nil {
			slog.Warn("previous forge webhook not removed", "connection", id, "error", err)
		}
	}
	if err := s.connections.SetWebhook(ctx, id, forge.Webhook{Mode: &mode, SecretEnc: &enc, HookID: hookID}); err != nil {
		return WebhookSetup{}, apperr.Wrap(err)
	}
	out := WebhookSetup{Mode: mode, URL: url}
	if mode == forge.WebhookManual {
		out.Secret = &secret
	}
	return out, nil
}

func (s *Service) DeleteWebhook(ctx context.Context, p access.Principal, nodeID, id uuid.UUID) error {
	c, err := s.authorizeConnection(ctx, p, catalog.PermAccess, nodeID, id)
	if err != nil {
		return err
	}
	sec, err := s.connections.Secrets(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if sec != nil && sec.WebhookID != nil {
		client, err := s.clientFor(ctx, c)
		if err != nil {
			return apperr.Wrap(err)
		}
		if err := client.DeleteHook(ctx, *sec.WebhookID); err != nil {
			return apperr.Wrap(err)
		}
	}
	return apperr.Wrap(s.connections.SetWebhook(ctx, id, forge.Webhook{}))
}

func (s *Service) ProjectReadme(ctx context.Context, p access.Principal, id uuid.UUID) (forge.Readme, error) {
	if _, err := s.catalog.Authorize(ctx, p, catalog.PermRead, id); err != nil {
		return forge.Readme{}, err
	}
	r, err := s.repositories.Readme(ctx, id)
	if err != nil {
		return forge.Readme{}, apperr.Wrap(err)
	}
	if r == nil {
		return forge.Readme{}, apperr.New(apperr.NotFound)
	}
	return *r, nil
}
