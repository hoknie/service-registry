package repositories

import (
	"github.com/jackc/pgx/v5"

	notifytypes "svc-registry/internal/feature/notify/types"
)

func scanSubscription(row pgx.CollectableRow) (notifytypes.Subscription, error) {
	var s notifytypes.Subscription
	var channel string
	var events []string
	err := row.Scan(&s.ID, &s.NodeID, &s.Key, &channel, &s.SecretID, &s.SecretName, &s.SigningEnc, &s.SigningFingerprint,
		&events, &s.Environments, &s.Locale, &s.Disabled, &s.CreatedAt, &s.UpdatedAt, &s.Inherited)
	s.Channel = notifytypes.Channel(channel)
	for _, e := range events {
		s.EventTypes = append(s.EventTypes, notifytypes.EventType(e))
	}
	return s, err
}

func scanDelivery(row pgx.CollectableRow) (notifytypes.Delivery, error) {
	var d notifytypes.Delivery
	var typ, channel, status string
	err := row.Scan(&d.ID, &d.EventID, &typ, &d.EventNodeID, &d.EventNodePath, &d.SubscriptionKey, &channel, &status,
		&d.Attempts, &d.NextAttemptAt, &d.LastHTTPStatus, &d.LastErrorCode, &d.LastErrorMessage, &d.CreatedAt, &d.DeliveredAt)
	d.EventType, d.Channel, d.Status = notifytypes.EventType(typ), notifytypes.Channel(channel), notifytypes.Status(status)
	return d, err
}

func scanJob(row pgx.CollectableRow) (notifytypes.Job, error) {
	var j notifytypes.Job
	var status string
	err := row.Scan(&j.ID, &j.SubscriptionID, &status, &j.Attempts, &j.Payload)
	j.Status = notifytypes.Status(status)
	return j, err
}

func eventTypes(in []notifytypes.EventType) []string {
	out := make([]string, len(in))
	for i, e := range in {
		out[i] = string(e)
	}
	return out
}
