package requests

import (
	"github.com/google/uuid"

	notifytypes "svc-registry/internal/feature/notify/types"
)

type Subscription struct {
	Channel      *string    `json:"channel" validate:"required"`
	SecretID     *uuid.UUID `json:"secret_id"`
	EventTypes   *[]string  `json:"event_types" validate:"required"`
	Environments *[]string  `json:"environments"`
	Locale       *string    `json:"locale"`
	Disabled     *bool      `json:"disabled"`
}

func (r Subscription) Put() notifytypes.PutSubscription {
	out := notifytypes.PutSubscription{Channel: *r.Channel, SecretID: r.SecretID, EventTypes: *r.EventTypes, Locale: r.Locale}
	if r.Environments != nil {
		out.Environments = *r.Environments
	}
	if r.Disabled != nil {
		out.Disabled = *r.Disabled
	}
	return out
}

type Deliveries struct {
	Page
	Subscription *string `query:"subscription"`
	Status       *string `query:"status"`
}

func (q Deliveries) Filter() notifytypes.DeliveryFilter {
	return notifytypes.DeliveryFilter{Subscription: q.Subscription, Status: q.Status}
}
