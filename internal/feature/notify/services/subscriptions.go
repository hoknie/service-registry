package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/notify/repositories"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

func (s *SubscriptionsService) ListSubscriptions(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID) ([]notifytypes.Subscription, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID); err != nil {
		return nil, err
	}
	out, err := s.subscriptionRepository.Effective(ctx, nodeID)
	return out, apperr.Wrap(err)
}

func (s *SubscriptionsService) PutSubscription(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID, key string, in notifytypes.PutSubscription) (notifytypes.Subscription, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID); err != nil {
		return notifytypes.Subscription{}, err
	}
	n, err := notifytypes.ValidateSubscription(key, in)
	if err != nil {
		return notifytypes.Subscription{}, apperr.Wrap(err)
	}
	if n.SecretID != nil {
		if err := s.checkSecret(ctx, nodeID, n.Channel, *n.SecretID); err != nil {
			return notifytypes.Subscription{}, err
		}
	}
	if err := s.put(ctx, nodeID, n); err != nil {
		return notifytypes.Subscription{}, apperr.Wrap(err)
	}
	own, err := s.own(ctx, nodeID, n.Key)
	if err != nil {
		return notifytypes.Subscription{}, apperr.Wrap(err)
	}
	if own == nil {
		return notifytypes.Subscription{}, apperr.New(apperr.NotFound)
	}
	return *own, nil
}

func (s *SubscriptionsService) put(ctx context.Context, nodeID uuid.UUID, n notifytypes.NewSubscription) error {
	return s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.subscriptionRepository.LockNode(ctx, nodeID); err != nil {
			return err
		}
		count, err := s.subscriptionRepository.CountOthers(ctx, nodeID, n.Key)
		if err != nil {
			return err
		}
		if count >= notifytypes.MaxSubscriptions {
			return notifytypes.TooManySubscriptions
		}
		return s.subscriptionRepository.Upsert(ctx, uuid.Must(uuid.NewV7()), nodeID, n)
	})
}

func (s *SubscriptionsService) checkSecret(ctx context.Context, nodeID uuid.UUID, channel notifytypes.Channel, id uuid.UUID) error {
	if _, err := s.catalog.SecretResolver.SecretFor(ctx, &nodeID, id); err != nil {
		return err
	}
	value, err := s.catalog.SecretResolver.ResolveSecret(ctx, id)
	if apperr.IsUnavailable(err) {
		return apperr.Wrap(err)
	}
	if err != nil || notifytypes.ValidateURL(channel, value) != nil {
		return apperr.Wrap(notifytypes.InvalidNotificationURL)
	}
	return nil
}

func (s *SubscriptionsService) DeleteSubscription(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID, key string) error {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID); err != nil {
		return err
	}
	k, ok := notifytypes.ValidateKey(key)
	if !ok {
		return apperr.New(apperr.NotFound)
	}
	return apperr.Wrap(s.subscriptionRepository.Delete(ctx, nodeID, k))
}

func (s *SubscriptionsService) RotateSigningSecret(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID, key string) (string, string, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID); err != nil {
		return "", "", err
	}
	own, err := s.own(ctx, nodeID, key)
	if err != nil {
		return "", "", apperr.Wrap(err)
	}
	if own == nil || own.Channel != notifytypes.ChannelWebhook {
		return "", "", apperr.New(apperr.NotFound)
	}
	if !s.secrets.CanEncrypt() {
		return "", "", apperr.Wrap(catalogtypes.ConflictSecretsKeyMissing)
	}
	secret, err := notifytypes.NewSigningSecret()
	if err != nil {
		return "", "", apperr.InternalDetail(err.Error())
	}
	enc, err := s.secrets.Seal(secret, secretbox.AAD(notifytypes.Table, own.ID.String(), notifytypes.ColumnSigning))
	if err != nil {
		return "", "", apperr.InternalDetail(err.Error())
	}
	fingerprint := notifytypes.Fingerprint(secret)
	if err := s.subscriptionRepository.SetSigning(ctx, own.ID, enc, fingerprint); err != nil {
		return "", "", apperr.Wrap(err)
	}
	return secret, fingerprint, nil
}

func (s *SubscriptionsService) ListDeliveries(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID, f notifytypes.DeliveryFilter, q accesstypes.PageQuery) (accesstypes.Page[notifytypes.Delivery], error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID); err != nil {
		return accesstypes.Page[notifytypes.Delivery]{}, err
	}
	if f.Status != nil && !notifytypes.ValidStatus(*f.Status) {
		return accesstypes.Page[notifytypes.Delivery]{}, apperr.Wrap(notifytypes.InvalidDeliveryFilter)
	}
	if f.Subscription != nil {
		k, ok := notifytypes.ValidateKey(*f.Subscription)
		if !ok {
			return accesstypes.Page[notifytypes.Delivery]{}, apperr.Wrap(notifytypes.InvalidDeliveryFilter)
		}
		f.Subscription = &k
	}
	page, err := accesstypes.ValidatePage(q)
	if err != nil {
		return accesstypes.Page[notifytypes.Delivery]{}, apperr.Wrap(err)
	}
	out, err := s.outboxRepository.Journal(ctx, nodeID, f, page)
	if err != nil {
		return accesstypes.Page[notifytypes.Delivery]{}, apperr.Wrap(err)
	}
	readable := map[uuid.UUID]bool{}
	for i, d := range out.Items {
		ok, seen := readable[d.EventNodeID]
		if !seen {
			sc, err := s.catalog.Access.Scope(ctx, p, d.EventNodeID)
			ok = err == nil && sc.Allows(catalogtypes.PermRead)
			readable[d.EventNodeID] = ok
		}
		if !ok {
			out.Items[i].EventNodePath = nil
		}
	}
	return out, nil
}

func (s *SubscriptionsService) own(ctx context.Context, nodeID uuid.UUID, key string) (*notifytypes.Subscription, error) {
	sub, err := s.effective(ctx, nodeID, key)
	if err != nil || sub == nil || sub.Inherited {
		return nil, err
	}
	return sub, nil
}

func (s *SubscriptionsService) effective(ctx context.Context, nodeID uuid.UUID, key string) (*notifytypes.Subscription, error) {
	k, ok := notifytypes.ValidateKey(key)
	if !ok {
		return nil, nil
	}
	subs, err := s.subscriptionRepository.Effective(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	for _, sub := range subs {
		if sub.Key == k {
			return &sub, nil
		}
	}
	return nil, nil
}

type SubscriptionsService struct {
	catalog                *catalogservices.CatalogServices
	db                     *pgtx.DB
	deliveries             *DeliveriesService
	outbox                 *OutboxService
	outboxRepository       *repositories.OutboxRepository
	secrets                *secretbox.Box
	subscriptionRepository *repositories.SubscriptionRepository
}
