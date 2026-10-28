package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/apperr"
)

func (s *ServersService) ServerTools(ctx context.Context, p accesstypes.Principal, id uuid.UUID) ([]gatewaytypes.ToolView, error) {
	if _, err := readServer(ctx, s.parts, p, id); err != nil {
		return nil, err
	}
	slug, tools, disabled, err := s.servers.Tools(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	return gatewaytypes.Views(slug, tools, disabled), nil
}

func (s *ServersService) DisableTools(ctx context.Context, p accesstypes.Principal, id uuid.UUID, names []string) ([]gatewaytypes.ToolView, error) {
	if _, err := writeServer(ctx, s.parts, p, id, accesstypes.ActionUpdate); err != nil {
		return nil, err
	}
	clean, err := gatewaytypes.CleanDisabledTools(names)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if err := s.servers.SetDisabledTools(ctx, id, clean); err != nil {
		return nil, apperr.Wrap(err)
	}
	return s.ServerTools(ctx, p, id)
}

func (s *ServersService) ServerIcon(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (gatewaytypes.IconFile, error) {
	if _, err := readServer(ctx, s.parts, p, id); err != nil {
		return gatewaytypes.IconFile{}, err
	}
	icon, err := s.checks.Icon(ctx, id)
	if err != nil {
		return gatewaytypes.IconFile{}, apperr.Wrap(err)
	}
	if icon == nil {
		return gatewaytypes.IconFile{}, apperr.New(apperr.NotFound)
	}
	return *icon, nil
}

func (s *ServersService) BuiltinDisabled(ctx context.Context) ([]string, error) {
	out, err := s.builtin.Disabled(ctx)
	return out, apperr.Wrap(err)
}

func (s *ServersService) BuiltinTools(ctx context.Context, p accesstypes.Principal) ([]string, bool, error) {
	disabled, err := s.BuiltinDisabled(ctx)
	return disabled, globalRight(p, accesstypes.ActionUpdate) == nil, err
}

func (s *ServersService) DisableBuiltinTools(ctx context.Context, p accesstypes.Principal, names []string) ([]string, bool, error) {
	if err := globalRight(p, accesstypes.ActionUpdate); err != nil {
		return nil, false, err
	}
	clean, err := gatewaytypes.CleanBuiltinTools(names)
	if err != nil {
		return nil, false, apperr.Wrap(err)
	}
	if err := s.builtin.SetDisabled(ctx, clean); err != nil {
		return nil, false, apperr.Wrap(err)
	}
	return s.BuiltinTools(ctx, p)
}
