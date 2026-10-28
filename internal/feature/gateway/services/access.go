package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/apperr"
)

type accessRules struct {
	catalog *catalogservices.CatalogServices
}

func globalRight(p accesstypes.Principal, action accesstypes.Action) error {
	if p.ByToken() && !p.Scopes.Has(accesstypes.ScopeMCPServers) {
		return apperr.New(apperr.InsufficientScope)
	}
	return accesstypes.RequireRight(p, accesstypes.SectionMCP, action)
}

func (a *accessRules) canRead(ctx context.Context, p accesstypes.Principal, s gatewaytypes.Scope) error {
	if s.Global() {
		return nil
	}
	if _, err := a.catalog.Access.Scope(ctx, p, *s.OrganizationID); err != nil {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func (a *accessRules) canWrite(ctx context.Context, p accesstypes.Principal, s gatewaytypes.Scope, action accesstypes.Action) error {
	if s.Global() {
		return globalRight(p, action)
	}
	_, err := a.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, *s.OrganizationID)
	return err
}

func (a *accessRules) canManage(ctx context.Context, p accesstypes.Principal, s gatewaytypes.Scope) error {
	if s.Global() {
		return globalRight(p, accesstypes.ActionUpdate)
	}
	_, err := a.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, *s.OrganizationID)
	return err
}

func (a *accessRules) writes(ctx context.Context, p accesstypes.Principal, s gatewaytypes.Scope) bool {
	return a.canWrite(ctx, p, s, accesstypes.ActionUpdate) == nil
}

func (a *accessRules) manages(ctx context.Context, p accesstypes.Principal, s gatewaytypes.Scope) bool {
	return a.canManage(ctx, p, s) == nil
}

func (a *accessRules) visible(ctx context.Context, p accesstypes.Principal) (gatewaytypes.Visible, gatewaytypes.Visible, error) {
	read, err := a.catalog.Access.VisibleOrganizations(ctx, p, catalogtypes.PermRead)
	if err != nil {
		return gatewaytypes.Visible{}, gatewaytypes.Visible{}, err
	}
	write, err := a.catalog.Access.VisibleOrganizations(ctx, p, catalogtypes.PermWrite)
	if err != nil {
		return gatewaytypes.Visible{}, gatewaytypes.Visible{}, err
	}
	return gatewaytypes.Visible{Organizations: read, Global: true},
		gatewaytypes.Visible{Organizations: write, Global: globalRight(p, accesstypes.ActionUpdate) == nil}, nil
}

func (a *accessRules) managed(ctx context.Context, p accesstypes.Principal) (gatewaytypes.Visible, error) {
	orgs, err := a.catalog.Access.VisibleOrganizations(ctx, p, catalogtypes.PermAccess)
	if err != nil {
		return gatewaytypes.Visible{}, err
	}
	return gatewaytypes.Visible{Organizations: orgs, Global: globalRight(p, accesstypes.ActionUpdate) == nil}, nil
}

func (a *accessRules) viewer(ctx context.Context, p accesstypes.Principal) (gatewaytypes.Viewer, error) {
	read, write, err := a.visible(ctx, p)
	if err != nil {
		return gatewaytypes.Viewer{}, err
	}
	v := gatewaytypes.Viewer{UserID: p.UserID, Read: read.Organizations, Write: write, Limited: p.MCPServers.Limited,
		Servers: p.MCPServers.IDs}
	if len(p.Nodes) > 0 {
		orgs, err := a.catalog.Access.NodeOrganizations(ctx, p.Nodes)
		if err != nil {
			return gatewaytypes.Viewer{}, err
		}
		v.Organizations = append([]uuid.UUID{}, orgs...)
	}
	return v, nil
}
