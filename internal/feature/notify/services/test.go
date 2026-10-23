package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/platform/apperr"
)

const testTimeout = 10 * time.Second

func (s *SubscriptionsService) TestSubscription(ctx context.Context, p accesstypes.Principal, nodeID uuid.UUID, key string) (notifytypes.TestResult, error) {
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermAccess, nodeID); err != nil {
		return notifytypes.TestResult{}, err
	}
	sub, err := s.effective(ctx, nodeID, key)
	if err != nil {
		return notifytypes.TestResult{}, apperr.Wrap(err)
	}
	if sub == nil {
		return notifytypes.TestResult{}, apperr.New(apperr.NotFound)
	}
	wait, err := s.subscriptionRepository.MarkTest(ctx, sub.ID, notifytypes.TestEverySecs)
	if err != nil {
		return notifytypes.TestResult{}, apperr.Wrap(err)
	}
	if wait > 0 {
		return notifytypes.TestResult{}, notifytypes.RateLimited(uint64(wait))
	}
	eventID := uuid.Must(uuid.NewV7())
	var deliveries []uuid.UUID
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		deliveries, err = s.outbox.record(ctx, notifytypes.Event{Type: notifytypes.EventTest, NodeID: nodeID,
			DedupeKey: notifytypes.TestKey(eventID), OccurredAt: time.Now(), Data: notifytypes.TestData{Subscription: sub.Key}},
			[]uuid.UUID{sub.ID}, nil)
		return err
	})
	if err != nil {
		return notifytypes.TestResult{}, apperr.Wrap(err)
	}
	if len(deliveries) != 1 {
		return notifytypes.TestResult{}, apperr.New(apperr.NotFound)
	}
	job, err := s.outboxRepository.Job(ctx, deliveries[0])
	if err != nil || job == nil {
		return notifytypes.TestResult{}, apperr.Wrap(err)
	}
	sendCtx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	r, err := s.deliveries.attempt(sendCtx, *sub, job.ID, job.Payload)
	if err != nil {
		return notifytypes.TestResult{}, err
	}
	o := notifytypes.Decide(r, 1, 1)
	if err := s.outboxRepository.Finish(context.WithoutCancel(ctx), job.ID, o); err != nil {
		return notifytypes.TestResult{}, apperr.Wrap(err)
	}
	return notifytypes.TestResult{Status: o.Status, HTTPStatus: r.HTTPStatus, ErrorCode: r.ErrorCode, DurationMS: r.DurationMS}, nil
}
