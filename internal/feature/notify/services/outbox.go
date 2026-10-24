package services

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	"svc-registry/internal/feature/notify/repositories"
	notifytypes "svc-registry/internal/feature/notify/types"
	ownershipservices "svc-registry/internal/feature/ownership/services"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

func (s *OutboxService) Emit(ctx context.Context, ev notifytypes.Event) error {
	subs, err := s.subscriptionRepository.Effective(ctx, ev.NodeID)
	if err != nil {
		return apperr.Wrap(err)
	}
	var ids []uuid.UUID
	for _, sub := range subs {
		if sub.Matches(ev.Type, ev.Environment) {
			ids = append(ids, sub.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if ev.Episode == "" {
		_, err = s.record(ctx, ev, ids, nil)
		return err
	}
	return apperr.Wrap(s.db.InTx(ctx, func(ctx context.Context) error {
		id := uuid.Must(uuid.NewV7())
		opened, err := s.episodeRepository.Open(ctx, notifytypes.NewEpisode{ID: uuid.Must(uuid.NewV7()), Key: ev.Episode,
			Type: ev.Type, NodeID: ev.NodeID, Signature: ev.DedupeKey}, id)
		if err != nil || !opened {
			return err
		}
		ev.DedupeKey = notifytypes.EpisodeEventKey(id)
		deliveries, err := s.recordAs(ctx, id, ev, ids, nil)
		if err != nil || deliveries != nil {
			return err
		}
		_, err = s.episodeRepository.Close(ctx, ev.Episode)
		return err
	}))
}

func (s *OutboxService) Recover(ctx context.Context, ev notifytypes.Event) error {
	return apperr.Wrap(s.db.InTx(ctx, func(ctx context.Context) error {
		failure, err := s.episodeRepository.Close(ctx, ev.Episode)
		if err != nil || failure == nil {
			return err
		}
		received, err := s.episodeRepository.Recipients(ctx, *failure)
		if err != nil || len(received) == 0 {
			return err
		}
		subs, err := s.subscriptionRepository.Effective(ctx, ev.NodeID)
		if err != nil {
			return err
		}
		var ids []uuid.UUID
		for _, sub := range subs {
			if sub.Matches(ev.Type, ev.Environment) && slices.Contains(received, sub.ID) {
				ids = append(ids, sub.ID)
			}
		}
		if len(ids) == 0 {
			return nil
		}
		ev.DedupeKey = notifytypes.RecoveryEventKey(*failure)
		_, err = s.record(ctx, ev, ids, failure)
		return err
	}))
}

func (s *OutboxService) OpenEpisodes(ctx context.Context, typ notifytypes.EventType) ([]notifytypes.Episode, error) {
	eps, err := s.episodeRepository.OpenOf(ctx, typ)
	return eps, apperr.Wrap(err)
}

func (s *OutboxService) record(ctx context.Context, ev notifytypes.Event, subscriptions []uuid.UUID, recovers *uuid.UUID) ([]uuid.UUID, error) {
	return s.recordAs(ctx, uuid.Must(uuid.NewV7()), ev, subscriptions, recovers)
}

func (s *OutboxService) recordAs(ctx context.Context, id uuid.UUID, ev notifytypes.Event, subscriptions []uuid.UUID, recovers *uuid.UUID) ([]uuid.UUID, error) {
	body, err := s.payload(ctx, id, ev, recovers)
	if err != nil || body == nil {
		return nil, err
	}
	var deliveries []uuid.UUID
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		inserted, err := s.outboxRepository.InsertEvent(ctx, notifytypes.NewEvent{ID: id, Type: ev.Type, NodeID: ev.NodeID,
			Payload: body, DedupeKey: ev.DedupeKey, OccurredAt: ev.OccurredAt})
		if err != nil || !inserted {
			return err
		}
		ids := make([]uuid.UUID, len(subscriptions))
		for i := range ids {
			ids[i] = uuid.Must(uuid.NewV7())
		}
		if err := s.outboxRepository.InsertDeliveries(ctx, id, ids, subscriptions); err != nil {
			return err
		}
		deliveries = ids
		return nil
	})
	return deliveries, apperr.Wrap(err)
}

func (s *OutboxService) payload(ctx context.Context, id uuid.UUID, ev notifytypes.Event, recovers *uuid.UUID) ([]byte, error) {
	chain, err := s.catalog.System.NodeChain(ctx, ev.NodeID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if chain == nil || len(chain.Nodes) == 0 {
		return nil, nil
	}
	node := chain.Nodes[0].Node
	slugs := make([]string, 0, len(chain.Nodes))
	for _, n := range chain.Nodes {
		slugs = append(slugs, n.Node.Slug)
	}
	slices.Reverse(slugs)
	owners, err := s.ownership.Ports.EffectiveOwners(ctx, ev.NodeID)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(ev.Data)
	if err != nil {
		return nil, apperr.InternalDetail(err.Error())
	}
	p := notifytypes.Payload{ID: id, Type: ev.Type, OccurredAt: ev.OccurredAt.UTC().Format(time.RFC3339),
		Node: notifytypes.NodeRef{ID: node.ID, Kind: string(node.Kind), Name: node.Name, Path: strings.Join(slugs, "/"),
			URL: notifytypes.NodeURL(s.cfg.Web.PublicURL, notifytypes.DefaultLocale, node.ID)},
		Data: data, Recovers: recovers}
	for _, o := range owners {
		owner := notifytypes.Owner{SubjectKind: string(o.SubjectKind), SubjectID: o.SubjectID, Name: o.Name, Email: o.Email,
			Status: o.Status, OnCallURL: o.OnCallURL, Contacts: []notifytypes.Contact{}}
		for _, c := range o.Contacts {
			owner.Contacts = append(owner.Contacts, notifytypes.Contact{Kind: c.Kind, Value: c.Value, Label: c.Label})
		}
		p.Owners = append(p.Owners, owner)
	}
	body, err := notifytypes.EncodePayload(p)
	if err != nil {
		return nil, apperr.InternalDetail(err.Error())
	}
	return body, nil
}

type OutboxService struct {
	catalog                *catalogservices.CatalogServices
	cfg                    *config.Config
	db                     *pgtx.DB
	episodeRepository      *repositories.EpisodeRepository
	ownership              *ownershipservices.OwnershipServices
	outboxRepository       *repositories.OutboxRepository
	subscriptionRepository *repositories.SubscriptionRepository
}
