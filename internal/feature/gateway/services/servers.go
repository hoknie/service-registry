package services

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/gateway/repositories"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/apperr"
)

type ServersService struct{ parts }

type ServerQuery struct {
	Query          string
	Tags           []string
	State          *string
	Status         *string
	FolderID       *uuid.UUID
	TopLevel       bool
	OrganizationID *uuid.UUID
	OnlyGlobal     bool
	Limit          int
	Cursor         string
}

type ServerPage struct {
	Items      []gatewaytypes.Server
	NextCursor *string
}

func (s *ServersService) ListServers(ctx context.Context, p accesstypes.Principal, q ServerQuery) (ServerPage, error) {
	read, write, err := s.access.visible(ctx, p)
	if err != nil {
		return ServerPage{}, err
	}
	if q.Limit == 0 {
		q.Limit = 50
	}
	f := repositories.ServerFilter{Read: read, Write: write, FolderID: q.FolderID, TopLevel: q.TopLevel,
		Recursive: q.FolderID != nil && (q.Query != "" || len(q.Tags) > 0 || q.State != nil), OrganizationID: q.OrganizationID,
		OnlyGlobal: q.OnlyGlobal, Query: strings.TrimSpace(q.Query), Tags: q.Tags, State: q.State, Status: q.Status, Limit: q.Limit + 1}
	if q.Cursor != "" {
		title, id, ok := decodeCursor(q.Cursor)
		if !ok {
			return ServerPage{}, apperr.Wrap(gatewaytypes.InvalidPagination)
		}
		f.AfterTitle, f.AfterID = &title, &id
	}
	found, err := s.servers.List(ctx, f)
	if err != nil {
		return ServerPage{}, apperr.Wrap(err)
	}
	page := ServerPage{Items: found}
	if len(found) > q.Limit {
		page.Items = found[:q.Limit]
		last := page.Items[q.Limit-1]
		page.NextCursor = new(encodeCursor(strings.ToLower(last.Title), last.ID))
	}
	managed, err := s.access.managed(ctx, p)
	if err != nil {
		return ServerPage{}, err
	}
	for i := range page.Items {
		page.Items[i].CanWrite = write.Has(page.Items[i].Scope())
		page.Items[i].CanManage = managed.Has(page.Items[i].Scope())
	}
	return page, nil
}

func encodeCursor(title string, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(title + "\x00" + id.String()))
}

func decodeCursor(raw string) (string, uuid.UUID, bool) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", uuid.UUID{}, false
	}
	title, rest, ok := strings.Cut(string(b), "\x00")
	id, err := uuid.Parse(rest)
	return title, id, ok && err == nil
}

func readServer(ctx context.Context, p parts, pr accesstypes.Principal, id uuid.UUID) (gatewaytypes.Server, error) {
	s, err := p.servers.Get(ctx, id)
	if err != nil {
		return gatewaytypes.Server{}, apperr.Wrap(err)
	}
	if s == nil {
		return gatewaytypes.Server{}, apperr.New(apperr.NotFound)
	}
	if err := p.access.canRead(ctx, pr, s.Scope()); err != nil {
		return gatewaytypes.Server{}, err
	}
	s.CanWrite = p.access.writes(ctx, pr, s.Scope())
	s.CanManage = p.access.manages(ctx, pr, s.Scope())
	if s.Status == gatewaytypes.StatusDraft && !s.CanWrite {
		return gatewaytypes.Server{}, apperr.New(apperr.NotFound)
	}
	if s.FolderID != nil {
		chain, err := folderChain(ctx, p, *s.FolderID)
		if err != nil {
			return gatewaytypes.Server{}, err
		}
		for i := len(chain) - 1; i >= 0; i-- {
			s.Ancestors = append(s.Ancestors, gatewaytypes.Ref{ID: chain[i].ID, Slug: chain[i].Slug, Name: chain[i].Name})
		}
	}
	return *s, nil
}

func writeServer(ctx context.Context, p parts, pr accesstypes.Principal, id uuid.UUID, action accesstypes.Action) (gatewaytypes.Server, error) {
	s, err := readServer(ctx, p, pr, id)
	if err != nil {
		return gatewaytypes.Server{}, err
	}
	if err := p.access.canWrite(ctx, pr, s.Scope(), action); err != nil {
		return gatewaytypes.Server{}, err
	}
	return s, nil
}

func (s *ServersService) GetServer(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (gatewaytypes.Server, error) {
	return readServer(ctx, s.parts, p, id)
}

type ServerInput struct {
	FolderID       *uuid.UUID
	OrganizationID *uuid.UUID
	Fields         gatewaytypes.ServerFields
}

func (s *ServersService) CreateServer(ctx context.Context, p accesstypes.Principal, in ServerInput) (gatewaytypes.Server, error) {
	in.Fields.Author = ""
	fields, err := gatewaytypes.CleanServer(in.Fields)
	if err != nil {
		return gatewaytypes.Server{}, apperr.Wrap(err)
	}
	if in.FolderID != nil {
		in.OrganizationID = nil
	}
	scope, err := placeOf(ctx, s.parts, p, in.FolderID, in.OrganizationID, accesstypes.ActionCreate)
	if err != nil {
		return gatewaytypes.Server{}, err
	}
	if err := s.checkSecret(ctx, scope, fields.Auth.SecretID); err != nil {
		return gatewaytypes.Server{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return gatewaytypes.Server{}, apperr.Wrap(err)
	}
	err = s.servers.Insert(ctx, repositories.NewServer{ID: id, FolderID: in.FolderID, OrganizationID: scope.OrganizationID,
		CreatedBy: p.UserID, Fields: fields})
	if err != nil {
		return gatewaytypes.Server{}, apperr.Wrap(repositories.Err(err))
	}
	return s.GetServer(ctx, p, id)
}

func (s *ServersService) checkSecret(ctx context.Context, scope gatewaytypes.Scope, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	_, err := s.catalog.SecretResolver.SecretFor(ctx, scope.OrganizationID, *id)
	return err
}

type ServerPatch struct {
	Slug        *string
	Title       *string
	Description *string
	Tags        *[]string
	IconURL     **string
	Icon        **string
	LogPayloads *bool
	URL         *string
	Transport   *string
	Auth        *gatewaytypes.Auth
	Status      *string
	Move        bool
	FolderID    *uuid.UUID
	OrgID       *uuid.UUID
}

func (s *ServersService) UpdateServer(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in ServerPatch) (gatewaytypes.Server, error) {
	cur, err := writeServer(ctx, s.parts, p, id, accesstypes.ActionUpdate)
	if err != nil {
		return gatewaytypes.Server{}, err
	}
	if in.Slug != nil && *in.Slug != cur.Slug {
		return gatewaytypes.Server{}, apperr.Wrap(gatewaytypes.SlugFixed)
	}
	fields, err := gatewaytypes.CleanServer(patched(cur, in))
	if err != nil {
		return gatewaytypes.Server{}, apperr.Wrap(err)
	}
	scope := cur.Scope()
	if in.Move {
		folder, org := in.FolderID, in.OrgID
		if folder != nil {
			org = nil
		}
		if scope, err = placeOf(ctx, s.parts, p, folder, org, accesstypes.ActionUpdate); err != nil {
			return gatewaytypes.Server{}, err
		}
	}
	if fields.Auth.SecretID != nil && (in.Auth != nil || in.Move) {
		if err := s.checkSecret(ctx, scope, fields.Auth.SecretID); err != nil {
			return gatewaytypes.Server{}, err
		}
	}
	recheck := fields.URL != cur.URL || fields.Transport != cur.Transport || in.Auth != nil || fields.Status != cur.Status
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.servers.Update(ctx, id, fields, recheck); err != nil {
			return err
		}
		if !in.Move {
			return nil
		}
		return s.servers.Move(ctx, id, in.FolderID, scope.OrganizationID)
	})
	if err != nil {
		return gatewaytypes.Server{}, apperr.Wrap(repositories.Err(err))
	}
	return s.GetServer(ctx, p, id)
}

func patched(cur gatewaytypes.Server, in ServerPatch) gatewaytypes.ServerFields {
	f := gatewaytypes.ServerFields{Slug: cur.Slug, Title: value(in.Title, cur.Title), Description: value(in.Description, cur.Description),
		Tags: cur.Tags, Author: cur.Author, IconURL: cur.IconURL, Icon: cur.Icon, LogPayloads: cur.LogPayloads, URL: value(in.URL, cur.URL),
		Transport: value(in.Transport, cur.Transport), Auth: gatewaytypes.Auth{Kind: cur.Auth.Kind, Header: cur.Auth.Header,
			SecretID: cur.Auth.SecretID}, Status: gatewaytypes.Status(value(in.Status, string(cur.Status)))}
	if in.Tags != nil {
		f.Tags = *in.Tags
	}
	if in.IconURL != nil {
		f.IconURL = *in.IconURL
	}
	if in.Icon != nil {
		f.Icon = *in.Icon
	}
	if in.LogPayloads != nil {
		f.LogPayloads = *in.LogPayloads
	}
	if in.Auth != nil {
		f.Auth = *in.Auth
	}
	return f
}

func (s *ServersService) DeleteServer(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if _, err := writeServer(ctx, s.parts, p, id, accesstypes.ActionDelete); err != nil {
		return err
	}
	return apperr.Wrap(s.servers.Delete(ctx, id))
}
