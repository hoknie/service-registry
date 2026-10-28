package services

import (
	"context"

	"github.com/google/uuid"

	accessports "svc-registry/internal/feature/access/ports"
	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
)

type AppsService struct {
	roots           accessports.NodeRootsPort
	servers         accessports.MCPServersPort
	grantRepository *repositories.OAuthGrantRepository
	userRepository  *repositories.UserRepository
}

func nodeRefs(ctx context.Context, roots accessports.NodeRootsPort, sets [][]uuid.UUID) ([][]accesstypes.NodeRef, error) {
	ids := []uuid.UUID{}
	for _, set := range sets {
		ids = append(ids, set...)
	}
	out := make([][]accesstypes.NodeRef, len(sets))
	if len(ids) == 0 {
		return out, nil
	}
	refs, err := roots.DescribeRoots(ctx, ids)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	for i, set := range sets {
		if set == nil {
			continue
		}
		out[i] = make([]accesstypes.NodeRef, len(set))
		for j, id := range set {
			ref, ok := refs[id]
			if !ok {
				ref = accesstypes.NodeRef{ID: id}
			}
			out[i][j] = ref
		}
	}
	return out, nil
}

func (s *AppsService) list(ctx context.Context, userID uuid.UUID) ([]accesstypes.AppGrant, error) {
	grants, err := s.grantRepository.ListForUser(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	sets := make([][]uuid.UUID, len(grants))
	for i, g := range grants {
		sets[i] = g.NodeIDs
	}
	refs, err := nodeRefs(ctx, s.roots, sets)
	if err != nil {
		return nil, err
	}
	for i := range grants {
		grants[i].Nodes = refs[i]
		if grants[i].MCPServers, err = describeServers(ctx, s.servers, grants[i].MCPServerSet); err != nil {
			return nil, err
		}
	}
	return grants, nil
}

func (s *AppsService) ListOwnApps(ctx context.Context, p accesstypes.Principal) ([]accesstypes.AppGrant, error) {
	if err := requireSession(p); err != nil {
		return nil, err
	}
	return s.list(ctx, p.UserID)
}

func (s *AppsService) RevokeOwnApp(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := requireSession(p); err != nil {
		return err
	}
	return apperr.Wrap(s.grantRepository.Revoke(ctx, id, new(p.UserID)))
}

func (s *AppsService) ListUserApps(ctx context.Context, p accesstypes.Principal, userID uuid.UUID) ([]accesstypes.AppGrant, error) {
	if err := requireSuperadminSession(p); err != nil {
		return nil, err
	}
	user, err := s.userRepository.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	return s.list(ctx, userID)
}

func (s *AppsService) RevokeUserApp(ctx context.Context, p accesstypes.Principal, userID, id uuid.UUID) error {
	if err := requireSuperadminSession(p); err != nil {
		return err
	}
	return apperr.Wrap(s.grantRepository.Revoke(ctx, id, new(userID)))
}

func allIn(list []uuid.UUID, set map[uuid.UUID]bool) bool {
	for _, n := range list {
		if !set[n] {
			return false
		}
	}
	return len(list) > 0
}

func checkServers(ctx context.Context, port accessports.MCPServersPort, owner accesstypes.Principal, set accesstypes.ServerSet) error {
	if !set.Limited || len(set.IDs) == 0 {
		return nil
	}
	if port == nil {
		return apperr.Wrap(accesstypes.InvalidMCPServers)
	}
	visible, err := port.VisibleServers(ctx, owner, set.IDs)
	if err != nil {
		return apperr.Wrap(err)
	}
	if !visible {
		return apperr.Wrap(accesstypes.InvalidMCPServers)
	}
	return nil
}

func describeServers(ctx context.Context, port accessports.MCPServersPort, set accesstypes.ServerSet) ([]accesstypes.ServerRef, error) {
	if !set.Limited {
		return nil, nil
	}
	if port == nil || len(set.IDs) == 0 {
		return []accesstypes.ServerRef{}, nil
	}
	refs, err := port.DescribeServers(ctx, set.IDs)
	return refs, apperr.Wrap(err)
}

func (s *AppsService) SetOwnAppServers(ctx context.Context, p accesstypes.Principal, id uuid.UUID, raw *[]string) (accesstypes.AppGrant, error) {
	if err := requireSession(p); err != nil {
		return accesstypes.AppGrant{}, err
	}
	if s.servers == nil {
		return accesstypes.AppGrant{}, apperr.New(apperr.NotFound)
	}
	owned, err := s.grantRepository.Owned(ctx, id, p.UserID)
	if err != nil {
		return accesstypes.AppGrant{}, apperr.Wrap(err)
	}
	if !owned {
		return accesstypes.AppGrant{}, apperr.New(apperr.NotFound)
	}
	set, err := accesstypes.ParseServerList(raw, accesstypes.Scopes{accesstypes.ScopeMCP})
	if err != nil {
		return accesstypes.AppGrant{}, apperr.Wrap(err)
	}
	if err := checkServers(ctx, s.servers, p, set); err != nil {
		return accesstypes.AppGrant{}, err
	}
	changed, err := s.grantRepository.SetServers(ctx, id, p.UserID, set.Stored())
	if err != nil {
		return accesstypes.AppGrant{}, apperr.Wrap(err)
	}
	if !changed {
		return accesstypes.AppGrant{}, apperr.Wrap(accesstypes.ConflictTokenNotActive)
	}
	grants, err := s.list(ctx, p.UserID)
	if err != nil {
		return accesstypes.AppGrant{}, err
	}
	for _, g := range grants {
		if g.ID == id {
			return g, nil
		}
	}
	return accesstypes.AppGrant{}, apperr.New(apperr.NotFound)
}

func (s *AuthorizationServerService) requestServers(ctx context.Context, p accesstypes.Principal) ([]accesstypes.ServerRef, error) {
	if s.servers == nil {
		return nil, nil
	}
	refs, err := s.servers.ListServers(ctx, p)
	return refs, apperr.Wrap(err)
}
