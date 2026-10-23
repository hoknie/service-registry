package responses

import (
	"github.com/google/uuid"

	notifytypes "svc-registry/internal/feature/notify/types"
)

type SubscriptionSecret struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type Signing struct {
	Fingerprint string `json:"fingerprint"`
}

type Subscription struct {
	Key          string              `json:"key"`
	Channel      string              `json:"channel"`
	SecretID     *uuid.UUID          `json:"secret_id"`
	Secret       *SubscriptionSecret `json:"secret"`
	EventTypes   []string            `json:"event_types"`
	Environments []string            `json:"environments"`
	Locale       string              `json:"locale"`
	Disabled     bool                `json:"disabled"`
	Signing      *Signing            `json:"signing"`
	NodeID       uuid.UUID           `json:"node_id"`
	Inherited    bool                `json:"inherited"`
	CreatedAt    string              `json:"created_at"`
	UpdatedAt    string              `json:"updated_at"`
}

func SubscriptionOf(s notifytypes.Subscription) Subscription {
	out := Subscription{Key: s.Key, Channel: string(s.Channel), SecretID: s.SecretID, EventTypes: make([]string, 0, len(s.EventTypes)),
		Environments: s.Environments, Locale: s.Locale, Disabled: s.Disabled, NodeID: s.NodeID, Inherited: s.Inherited,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
	for _, e := range s.EventTypes {
		out.EventTypes = append(out.EventTypes, string(e))
	}
	if s.SecretID != nil && s.SecretName != nil {
		out.Secret = &SubscriptionSecret{ID: *s.SecretID, Name: *s.SecretName}
	}
	if s.Channel == notifytypes.ChannelWebhook && s.SigningFingerprint != nil {
		out.Signing = &Signing{Fingerprint: *s.SigningFingerprint}
	}
	return out
}

type SigningSecret struct {
	Secret      string `json:"secret"`
	Fingerprint string `json:"fingerprint"`
}

type TestResult struct {
	Status     string  `json:"status"`
	HTTPStatus *int    `json:"http_status"`
	ErrorCode  *string `json:"error_code"`
	DurationMS int     `json:"duration_ms"`
}

func TestResultOf(r notifytypes.TestResult) TestResult {
	return TestResult{Status: string(r.Status), HTTPStatus: r.HTTPStatus, ErrorCode: r.ErrorCode, DurationMS: r.DurationMS}
}

type EventNode struct {
	ID   uuid.UUID `json:"id"`
	Path *string   `json:"path,omitempty"`
}

type Delivery struct {
	ID               uuid.UUID `json:"id"`
	EventID          uuid.UUID `json:"event_id"`
	EventType        string    `json:"event_type"`
	EventNode        EventNode `json:"event_node"`
	Subscription     string    `json:"subscription"`
	Channel          string    `json:"channel"`
	Status           string    `json:"status"`
	Attempts         int32     `json:"attempts"`
	NextAttemptAt    *string   `json:"next_attempt_at"`
	LastHTTPStatus   *int32    `json:"last_http_status"`
	LastErrorCode    *string   `json:"last_error_code"`
	LastErrorMessage *string   `json:"last_error_message"`
	CreatedAt        string    `json:"created_at"`
	DeliveredAt      *string   `json:"delivered_at"`
}

func DeliveryOf(d notifytypes.Delivery) Delivery {
	return Delivery{ID: d.ID, EventID: d.EventID, EventType: string(d.EventType), EventNode: EventNode{ID: d.EventNodeID, Path: d.EventNodePath},
		Subscription: d.SubscriptionKey, Channel: string(d.Channel), Status: string(d.Status), Attempts: d.Attempts,
		NextAttemptAt: d.NextAttemptAt, LastHTTPStatus: d.LastHTTPStatus, LastErrorCode: d.LastErrorCode,
		LastErrorMessage: d.LastErrorMessage, CreatedAt: d.CreatedAt, DeliveredAt: d.DeliveredAt}
}
