package services

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	gatewayports "svc-registry/internal/feature/gateway/ports"
	"svc-registry/internal/feature/gateway/repositories"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/feature/gateway/usecases"
	"svc-registry/internal/platform/apperr"
)

type GatewayService struct {
	parts
	call *usecases.CallToolUseCase
}

type GatewayTool struct {
	ServerID     uuid.UUID
	Name         string
	Title        string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	Annotations  json.RawMessage
}

type Offer struct {
	Server gatewaytypes.Server
	Tools  []gatewaytypes.ToolView
}

func (s *GatewayService) available(ctx context.Context, p accesstypes.Principal, slug *string) ([]Offer, error) {
	v, err := s.access.viewer(ctx, p)
	if err != nil {
		return nil, err
	}
	found, err := s.sets.Available(ctx, v, slug)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make([]Offer, len(found))
	for i, a := range found {
		out[i] = Offer{Server: a.Server, Tools: gatewaytypes.Offered(a.Server.Slug, a.Tools, a.Server.DisabledTools)}
	}
	return out, nil
}

func (s *GatewayService) Servers(ctx context.Context, p accesstypes.Principal, query string) ([]Offer, error) {
	all, err := s.available(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]Offer, 0, len(all))
	for _, o := range all {
		text := strings.ToLower(o.Server.Slug + " " + o.Server.Title + " " + o.Server.Description)
		if query == "" || strings.Contains(text, query) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (s *GatewayService) Tools(ctx context.Context, p accesstypes.Principal) ([]GatewayTool, error) {
	all, err := s.available(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	var out []GatewayTool
	for _, o := range all {
		for _, t := range o.Tools {
			out = append(out, GatewayTool{ServerID: o.Server.ID, Name: t.GatewayName, Title: gatewaytypes.GatewayTitle(t.Tool, o.Server.Title),
				Description: gatewaytypes.GatewayDescription(t.Tool, o.Server.Title), InputSchema: t.InputSchema,
				OutputSchema: t.OutputSchema, Annotations: t.Annotations})
		}
	}
	return out, nil
}

func (s *GatewayService) Call(ctx context.Context, p accesstypes.Principal, name string, args json.RawMessage) (*gatewayports.CallResult, error) {
	slug, tool, ok := gatewaytypes.SplitGatewayName(name)
	if !ok {
		return nil, apperr.New(apperr.NotFound)
	}
	found, err := s.available(ctx, p, &slug)
	if err != nil {
		return nil, err
	}
	if len(found) != 1 || !offers(found[0].Tools, tool) {
		return nil, apperr.New(apperr.NotFound)
	}
	srv := found[0].Server
	b, targetErr := s.targets.of(ctx, srv)
	caller := gatewaytypes.Caller{UserID: p.UserID}
	if p.ByToken() {
		id := p.TokenID
		if p.App {
			caller.GrantID = &id
		} else {
			caller.TokenID = &id
		}
	}
	return s.call.Run(ctx, usecases.CallInput{ServerID: srv.ID, Tool: tool, Args: args, Caller: caller, Target: b.target,
		TargetErr: targetErr, Secrets: b.secrets, Log: srv.LogPayloads})
}

func offers(tools []gatewaytypes.ToolView, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

type CallQuery struct {
	Tool    *string
	Outcome *string
	UserID  *uuid.UUID
	Limit   int
	Cursor  string
}

type CallPage struct {
	Items      []gatewaytypes.CallEntry
	NextCursor *string
}

func (s *GatewayService) CallLog(ctx context.Context, p accesstypes.Principal, id uuid.UUID, q CallQuery) (CallPage, error) {
	srv, err := readServer(ctx, s.parts, p, id)
	if err != nil {
		return CallPage{}, err
	}
	if !srv.CanWrite && !srv.CanManage {
		return CallPage{}, apperr.New(apperr.Forbidden)
	}
	if q.Limit == 0 {
		q.Limit = 50
	}
	f := repositories.CallFilter{ServerID: id, Tool: q.Tool, Outcome: q.Outcome, UserID: q.UserID, Limit: q.Limit + 1}
	if q.Cursor != "" {
		at, after, ok := decodeCursor(q.Cursor)
		if !ok {
			return CallPage{}, apperr.Wrap(gatewaytypes.InvalidPagination)
		}
		f.AfterAt, f.AfterID = &at, &after
	}
	found, err := s.calls.List(ctx, f)
	if err != nil {
		return CallPage{}, apperr.Wrap(err)
	}
	page := CallPage{Items: found}
	if len(found) > q.Limit {
		page.Items = found[:q.Limit]
		last := page.Items[q.Limit-1]
		page.NextCursor = new(encodeCursor(last.Cursor, last.ID))
	}
	return page, nil
}
