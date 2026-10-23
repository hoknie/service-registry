package repositories

import (
	catalogtypes "svc-registry/internal/feature/catalog/types"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"notification_subscriptions_node_id_fkey":              apperr.ErrNotFound,
	"notification_subscriptions_secret_id_fkey":            catalogtypes.InvalidSecretNotAvailable,
	"notification_subscriptions_key_check":                 notifytypes.InvalidSubscription,
	"notification_subscriptions_channel_check":             notifytypes.InvalidSubscription,
	"notification_subscriptions_locale_check":              notifytypes.InvalidSubscription,
	"notification_subscriptions_event_types_check":         notifytypes.InvalidSubscription,
	"notification_subscriptions_environments_check":        notifytypes.InvalidSubscription,
	"notification_subscriptions_secret_check":              notifytypes.InvalidSubscription,
	"notification_subscriptions_signing_check":             notifytypes.InvalidSubscription,
	"notification_subscriptions_signing_fingerprint_check": notifytypes.InvalidSubscription,
	"notification_events_node_id_fkey":                     apperr.ErrNotFound,
	"notification_episodes_node_id_fkey":                   apperr.ErrNotFound,
	"link_target_projects_link_key_check":                  notifytypes.InvalidSubscription,
}
