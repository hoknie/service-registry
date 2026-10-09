package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/links"
)

const projectPageSize = 100

func ClaimLinkChecks(ctx context.Context, state *State, limit, leaseSecs int) ([]links.Target, error) {
	items, err := state.LinkTargets.Claim(ctx, limit, leaseSecs)
	return items, apperr.Wrap(err)
}

func ExtendLinkCheck(ctx context.Context, state *State, t links.Target, leaseSecs int) error {
	return apperr.Wrap(state.LinkTargets.Extend(ctx, t.ID, leaseSecs))
}

func RunLinkCheck(ctx context.Context, state *State, t links.Target) error {
	r := state.LinkChecker.Check(ctx, t.URL)
	if ctx.Err() != nil {
		return nil
	}
	cfg := state.Config.LinkCheck
	_, err := state.LinkTargets.Record(ctx, t.URL, r, cfg.IntervalSecs, cfg.History)
	return apperr.Wrap(err)
}

func RefreshLinkTargets(ctx context.Context, state *State) (int, error) {
	seen := 0
	after := uuid.Nil
	for {
		ids, err := state.LinkTemplates.Projects(ctx, after, projectPageSize)
		if err != nil {
			return seen, apperr.Wrap(err)
		}
		for _, id := range ids {
			urls, err := projectURLs(ctx, state, id)
			if err != nil {
				return seen, err
			}
			if err := state.LinkTargets.Seen(ctx, urls); err != nil {
				return seen, apperr.Wrap(err)
			}
			seen += len(urls)
		}
		if len(ids) < projectPageSize {
			return seen, nil
		}
		after = ids[len(ids)-1]
	}
}

func projectURLs(ctx context.Context, state *State, id uuid.UUID) ([]string, error) {
	chain, err := state.Nodes.Chain(ctx, uuid.Nil, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if chain == nil || len(chain.Nodes) == 0 {
		return nil, nil
	}
	ancestors := make([]catalog.Node, 0, len(chain.Nodes)-1)
	for i := len(chain.Nodes) - 1; i >= 1; i-- {
		ancestors = append(ancestors, chain.Nodes[i].Node)
	}
	c, templates, err := loadContext(ctx, state, chain.Nodes[0].Node, ancestors)
	if err != nil {
		return nil, err
	}
	return links.URLs(links.Expand(c, templates, "")), nil
}

func PruneLinkTargets(ctx context.Context, state *State) (int64, error) {
	n, err := state.LinkTargets.Prune(ctx, state.Config.LinkCheck.TargetTTLDays)
	return n, apperr.Wrap(err)
}
