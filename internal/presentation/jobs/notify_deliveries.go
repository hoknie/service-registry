package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"svc-registry/pkg/leasejobs"
)

var (
	NotifyDeliveryTick     = 5 * time.Second
	NotifyRetentionEvery   = time.Hour
	notifyDeliveryLeaseFor = 2 * time.Minute
)

type deliverySource struct{ deps Deps }

func (s deliverySource) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return s.deps.Notify.Deliveries.ClaimDeliveries(ctx, limit, leaseSecs)
}

func (s deliverySource) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return s.deps.Notify.Deliveries.ExtendDelivery(ctx, id, leaseSecs)
}

func SpawnNotifyDeliveries(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	retention := make(chan struct{})
	go func() {
		defer close(retention)
		pruneNotifications(ctx, deps)
	}()
	if !deps.Config.Jobs.Enabled {
		go func() {
			defer close(done)
			<-retention
		}()
		return done
	}
	s := &leasejobs.Scheduler[uuid.UUID]{
		Name:    "notify-delivery",
		Source:  deliverySource{deps: deps},
		OnPanic: panicked(deps, "notify_deliveries"),
		Run: func(ctx context.Context, id uuid.UUID) {
			if err := observe(deps, "notify_deliveries", func() error { return deps.Notify.Deliveries.RunDelivery(ctx, id) }); err != nil {
				slog.Warn("notification delivery not recorded", "delivery", id, "error", err)
			}
		},
		Concurrency: int(deps.Config.Notify.Concurrency),
		Tick:        NotifyDeliveryTick,
		Lease:       notifyDeliveryLeaseFor,
	}
	go func() {
		defer close(done)
		s.Serve(ctx)
		<-retention
	}()
	return done
}

func pruneNotifications(ctx context.Context, deps Deps) {
	tick := time.NewTicker(NotifyRetentionEvery)
	defer tick.Stop()
	for {
		switch n, err := observeCount(deps, "notify_retention", func() (int64, error) { return deps.Notify.Deliveries.PruneNotifications(ctx) }); {
		case ctx.Err() != nil:
			return
		case err != nil:
			slog.Warn("notification retention failed; retrying at the next tick", "error", err)
		case n > 0:
			slog.Info("notification retention: old events deleted", "deleted", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
