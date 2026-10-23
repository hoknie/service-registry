package types

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

type Channel string

const (
	ChannelWebhook Channel = "webhook"
	ChannelSlack   Channel = "slack"
)

const (
	Table            = "notification_subscriptions"
	ColumnSigning    = "signing_secret_enc"
	TestEverySecs    = 10
	MaxSubscriptions = 20
	MaxEnvironments  = 20
	MaxURLLen        = 2048
	DefaultLocale    = "en"
)

var (
	Locales       = []string{"en", "es", "ru", "zh"}
	keyRe         = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	environmentRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$`)
)

type Subscription struct {
	ID                 uuid.UUID
	NodeID             uuid.UUID
	Key                string
	Channel            Channel
	SecretID           *uuid.UUID
	SecretName         *string
	SigningEnc         *string
	SigningFingerprint *string
	EventTypes         []EventType
	Environments       []string
	Locale             string
	Disabled           bool
	CreatedAt          string
	UpdatedAt          string
	Inherited          bool
}

type PutSubscription struct {
	Channel      string
	SecretID     *uuid.UUID
	EventTypes   []string
	Environments []string
	Locale       *string
	Disabled     bool
}

type NewSubscription struct {
	Key          string
	Channel      Channel
	SecretID     *uuid.UUID
	EventTypes   []EventType
	Environments []string
	Locale       string
	Disabled     bool
}

func ValidateKey(raw string) (string, bool) {
	k := strings.ToLower(strings.TrimSpace(raw))
	return k, keyRe.MatchString(k)
}

func ValidateSubscription(key string, in PutSubscription) (NewSubscription, error) {
	k, ok := ValidateKey(key)
	if !ok {
		return NewSubscription{}, InvalidSubscription
	}
	out := NewSubscription{Key: k, Channel: Channel(in.Channel), SecretID: in.SecretID, Locale: DefaultLocale, Disabled: in.Disabled}
	if out.Channel != ChannelWebhook && out.Channel != ChannelSlack {
		return NewSubscription{}, InvalidSubscription
	}
	if in.SecretID == nil && !in.Disabled {
		return NewSubscription{}, InvalidSubscription
	}
	if in.Locale != nil {
		if !slices.Contains(Locales, *in.Locale) {
			return NewSubscription{}, InvalidSubscription
		}
		out.Locale = *in.Locale
	}
	if len(in.EventTypes) == 0 || len(in.EventTypes) > len(SubscribableEvents) {
		return NewSubscription{}, InvalidSubscription
	}
	for _, t := range in.EventTypes {
		e := EventType(t)
		if !slices.Contains(SubscribableEvents, e) || slices.Contains(out.EventTypes, e) {
			return NewSubscription{}, InvalidSubscription
		}
		out.EventTypes = append(out.EventTypes, e)
	}
	for failure, recovery := range RecoveryOf {
		if slices.Contains(out.EventTypes, recovery) && !slices.Contains(out.EventTypes, failure) {
			return NewSubscription{}, RecoveryWithoutFailure
		}
	}
	if len(in.Environments) > 0 {
		if len(in.Environments) > MaxEnvironments {
			return NewSubscription{}, InvalidSubscription
		}
		out.Environments = []string{}
		for _, raw := range in.Environments {
			e := strings.ToLower(strings.TrimFunc(raw, unicode.IsSpace))
			if !environmentRe.MatchString(e) {
				return NewSubscription{}, InvalidSubscription
			}
			if !slices.Contains(out.Environments, e) {
				out.Environments = append(out.Environments, e)
			}
		}
	}
	return out, nil
}

func ValidateURL(channel Channel, raw string) error {
	if len(raw) > MaxURLLen || strings.IndexFunc(raw, unicode.IsControl) >= 0 || strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return InvalidNotificationURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return InvalidNotificationURL
	}
	if channel == ChannelSlack && (u.Scheme != "https" || !strings.EqualFold(u.Hostname(), "hooks.slack.com") || u.Port() != "") {
		return InvalidNotificationURL
	}
	return nil
}

func (s Subscription) Matches(t EventType, environment *string) bool {
	if s.Disabled || !slices.Contains(s.EventTypes, t) {
		return false
	}
	return s.Environments == nil || environment == nil || slices.Contains(s.Environments, *environment)
}

func Fingerprint(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])[:4]
}

type SigningRow struct {
	ID  uuid.UUID
	Enc string
}
