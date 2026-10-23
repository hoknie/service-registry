package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	notifyports "svc-registry/internal/feature/notify/ports"
	"svc-registry/internal/feature/notify/repositories"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/secretbox"
)

func (s *DeliveriesService) ClaimDeliveries(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	ids, err := s.outboxRepository.Claim(ctx, limit, leaseSecs)
	return ids, apperr.Wrap(err)
}

func (s *DeliveriesService) ExtendDelivery(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(s.outboxRepository.Extend(ctx, id, leaseSecs))
}

func (s *DeliveriesService) RunDelivery(ctx context.Context, id uuid.UUID) error {
	job, err := s.outboxRepository.Job(ctx, id)
	if err != nil || job == nil || job.Status != notifytypes.StatusPending {
		return apperr.Wrap(err)
	}
	sub, err := s.subscriptionRepository.Get(ctx, job.SubscriptionID)
	if err != nil || sub == nil {
		return apperr.Wrap(err)
	}
	var r notifytypes.Response
	if sub.Disabled {
		r = failure(notifytypes.CodeSubscriptionDisabled, "the subscription is disabled")
	} else if r, err = s.attempt(ctx, *sub, job.ID, job.Payload); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return nil
	}
	o := notifytypes.Decide(r, int(job.Attempts)+1, int(s.cfg.Notify.MaxAttempts))
	return apperr.Wrap(s.outboxRepository.Finish(context.WithoutCancel(ctx), id, o))
}

func (s *DeliveriesService) PruneNotifications(ctx context.Context) (int64, error) {
	n, err := s.outboxRepository.Prune(ctx, s.cfg.Notify.RetentionDays)
	return n, apperr.Wrap(err)
}

func (s *DeliveriesService) attempt(ctx context.Context, sub notifytypes.Subscription, deliveryID uuid.UUID, stored []byte) (notifytypes.Response, error) {
	if sub.SecretID == nil {
		return failure(notifytypes.CodeURLUnavailable, "the subscription has no address secret"), nil
	}
	target, err := s.catalog.SecretResolver.ResolveSecret(ctx, *sub.SecretID)
	if errors.Is(err, apperr.ErrUnavailable) || apperr.IsUnavailable(err) {
		return notifytypes.Response{}, apperr.Wrap(err)
	}
	if err != nil {
		return failure(notifytypes.CodeURLUnavailable, "the address secret cannot be read"), nil
	}
	if notifytypes.ValidateURL(sub.Channel, target) != nil {
		return failure(notifytypes.CodeURLUnavailable, "the address secret does not hold a valid address"), nil
	}
	var p notifytypes.Payload
	if err := json.Unmarshal(stored, &p); err != nil {
		return notifytypes.Response{}, apperr.InternalDetail(err.Error())
	}
	p = p.ForLocale(sub.Locale)
	req, err := s.request(sub, target, deliveryID, p)
	if err != nil {
		return failure(notifytypes.CodeURLUnavailable, "the signing key cannot be read"), nil
	}
	return s.sender.Send(ctx, req), nil
}

func (s *DeliveriesService) request(sub notifytypes.Subscription, target string, deliveryID uuid.UUID, p notifytypes.Payload) (notifytypes.Request, error) {
	agent := "svc-registry/" + s.version
	if sub.Channel == notifytypes.ChannelSlack {
		body, err := notifytypes.SlackBody(p, sub.Locale)
		if err != nil {
			return notifytypes.Request{}, err
		}
		return notifytypes.Request{URL: target, Body: body, Headers: map[string]string{"Content-Type": "application/json", "User-Agent": agent}}, nil
	}
	body, err := notifytypes.EncodePayload(p)
	if err != nil {
		return notifytypes.Request{}, err
	}
	var signing *string
	if sub.SigningEnc != nil {
		key, err := s.secrets.Open(*sub.SigningEnc, secretbox.AAD(notifytypes.Table, sub.ID.String(), notifytypes.ColumnSigning))
		if err != nil {
			return notifytypes.Request{}, err
		}
		signing = &key
	}
	return notifytypes.WebhookRequest(target, deliveryID, body, signing, time.Now(), agent)
}

func failure(code, message string) notifytypes.Response {
	return notifytypes.Response{ErrorCode: &code, ErrorMessage: &message}
}

type DeliveriesService struct {
	catalog                *catalogservices.CatalogServices
	cfg                    *config.Config
	outboxRepository       *repositories.OutboxRepository
	secrets                *secretbox.Box
	sender                 notifyports.SenderPort
	subscriptionRepository *repositories.SubscriptionRepository
	version                string
}
