package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	linkstypes "svc-registry/internal/feature/links/types"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/platform/apperr"
)

func (s *ProjectsService) record(ctx context.Context, url string, r linkstypes.Result) (linkstypes.Check, error) {
	cfg := s.cfg.LinkCheck
	var out linkstypes.Recorded
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		if out, err = s.targetRepository.Record(ctx, url, r, cfg.IntervalSecs, cfg.History); err != nil {
			return err
		}
		return s.emitTransition(ctx, out)
	})
	return out.Check, apperr.Wrap(err)
}

func (s *ProjectsService) emitTransition(ctx context.Context, rec linkstypes.Recorded) error {
	if rec.Previous == nil {
		return nil
	}
	previous, status := string(*rec.Previous), string(rec.Check.Status)
	failed := notifytypes.WorkingLink(previous) && notifytypes.BrokenLink(status)
	recovered := !notifytypes.WorkingLink(previous) && notifytypes.WorkingLink(status)
	if !failed && !recovered {
		return nil
	}
	pairs, err := s.targetRepository.Projects(ctx, rec.TargetID)
	if err != nil {
		return err
	}
	at, err := time.Parse(time.RFC3339, rec.Check.CheckedAt)
	if err != nil {
		at = time.Now()
	}
	for _, p := range pairs {
		link, err := s.systemLink(ctx, p.ProjectID, p.LinkKey, rec.Check.URL)
		if err != nil {
			return err
		}
		if link == nil {
			continue
		}
		ev := notifytypes.Event{Type: notifytypes.EventLinkCheckFailed, NodeID: p.ProjectID,
			DedupeKey: notifytypes.LinkKey(rec.TargetID, p.ProjectID, p.LinkKey, rec.CheckID),
			Episode:   notifytypes.LinkEpisode(rec.TargetID, p.ProjectID, p.LinkKey), OccurredAt: at,
			Data: notifytypes.LinkData{LinkKey: link.LinkKey, KindKey: link.KindKey, Title: link.Title, URL: rec.Check.URL,
				Status: status, HTTPStatus: rec.Check.HTTPStatus, PreviousStatus: previous, CheckedAt: rec.Check.CheckedAt}}
		if recovered {
			ev.Type = notifytypes.EventLinkRecovered
			err = s.notify.Outbox.Recover(ctx, ev)
		} else {
			err = s.notify.Outbox.Emit(ctx, ev)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ProjectsService) systemLink(ctx context.Context, projectID uuid.UUID, linkKey, url string) (*linkstypes.Link, error) {
	links, err := s.systemLinks(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, l := range links {
		if l.LinkKey == linkKey && l.URL != nil && *l.URL == url {
			return &l, nil
		}
	}
	return nil, nil
}

func (s *ProjectsService) systemLinks(ctx context.Context, id uuid.UUID) ([]linkstypes.Link, error) {
	chain, err := s.catalog.System.NodeChain(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if chain == nil || len(chain.Nodes) == 0 {
		return nil, nil
	}
	ancestors := make([]catalogtypes.Node, 0, len(chain.Nodes)-1)
	for i := len(chain.Nodes) - 1; i >= 1; i-- {
		ancestors = append(ancestors, chain.Nodes[i].Node)
	}
	c, templates, err := s.loadContext(ctx, chain.Nodes[0].Node, ancestors)
	if err != nil {
		return nil, err
	}
	return linkstypes.ExpandProject(c, templates, ""), nil
}
