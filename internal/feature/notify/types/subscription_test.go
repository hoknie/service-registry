package types

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestValidateSubscription(t *testing.T) {
	t.Parallel()
	secret := uuid.Must(uuid.NewV7())
	ok := PutSubscription{Channel: "webhook", SecretID: &secret, EventTypes: []string{"deployment.created"}}
	got, err := ValidateSubscription(" Ops ", ok)
	if err != nil || got.Key != "ops" || got.Locale != "en" || got.Environments != nil {
		t.Fatalf("%+v %v", got, err)
	}
	envs := ok
	envs.Environments = []string{"Production", "production", "staging"}
	got, err = ValidateSubscription("ops", envs)
	if err != nil || !slices.Equal(got.Environments, []string{"production", "staging"}) {
		t.Fatalf("%+v %v", got, err)
	}
	for name, mutate := range map[string]func(*PutSubscription){
		"channel":        func(p *PutSubscription) { p.Channel = "email" },
		"no secret":      func(p *PutSubscription) { p.SecretID = nil },
		"no events":      func(p *PutSubscription) { p.EventTypes = nil },
		"unknown event":  func(p *PutSubscription) { p.EventTypes = []string{"test"} },
		"repeated event": func(p *PutSubscription) { p.EventTypes = []string{"link.check_failed", "link.check_failed"} },
		"locale":         func(p *PutSubscription) { p.Locale = new("de") },
		"environment":    func(p *PutSubscription) { p.Environments = []string{"prod uction"} },
		"many envs":      func(p *PutSubscription) { p.Environments = make([]string, 21) },
	} {
		in := ok
		mutate(&in)
		if _, err := ValidateSubscription("ops", in); !errors.Is(err, InvalidSubscription) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := ValidateSubscription("-ops", ok); !errors.Is(err, InvalidSubscription) {
		t.Errorf("key: %v", err)
	}
	for _, events := range [][]string{{"deployment.drift_resolved"}, {"link.check_recovered", "deployment.drift"},
		{"forge.sync_recovered", "link.check_failed"}} {
		in := ok
		in.EventTypes = events
		if _, err := ValidateSubscription("ops", in); !errors.Is(err, RecoveryWithoutFailure) {
			t.Errorf("%v: %v", events, err)
		}
	}
	paired := ok
	paired.EventTypes = []string{"link.check_recovered", "link.check_failed", "forge.sync_failed"}
	if _, err := ValidateSubscription("ops", paired); err != nil {
		t.Errorf("paired: %v", err)
	}
	gone := PutSubscription{Channel: "slack", EventTypes: []string{"forge.sync_recovered"}, Disabled: true}
	if _, err := ValidateSubscription("ops", gone); !errors.Is(err, RecoveryWithoutFailure) {
		t.Errorf("disabled recovery only: %v", err)
	}
	disabled := PutSubscription{Channel: "slack", EventTypes: []string{"deployment.drift"}, Disabled: true}
	if _, err := ValidateSubscription("ops", disabled); err != nil {
		t.Errorf("a disabled subscription needs no secret: %v", err)
	}
}

func TestValidateURL(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]bool{
		"https://hooks.example/x":                             true,
		"http://10.0.0.5:8080/hook":                           true,
		"ftp://hooks.example/x":                               false,
		"https://user:pw@hooks.example/x":                     false,
		"/relative":                                           false,
		"https://hooks.example/a b":                           false,
		"https://hooks.example/" + string(make([]byte, 2048)): false,
	} {
		if got := ValidateURL(ChannelWebhook, raw) == nil; got != want {
			t.Errorf("webhook %q: %v", raw, got)
		}
	}
	for raw, want := range map[string]bool{
		"https://hooks.slack.com/services/T/B/x": true,
		"https://chat.example/hook":              false,
		"http://hooks.slack.com/services/T/B/x":  false,
	} {
		if got := ValidateURL(ChannelSlack, raw) == nil; got != want {
			t.Errorf("slack %q: %v", raw, got)
		}
	}
}

func TestMatches(t *testing.T) {
	t.Parallel()
	prod := "production"
	staging := "staging"
	s := Subscription{EventTypes: []EventType{EventDeploymentCreated, EventLinkCheckFailed}, Environments: []string{prod}}
	if !s.Matches(EventDeploymentCreated, &prod) || s.Matches(EventDeploymentCreated, &staging) ||
		!s.Matches(EventLinkCheckFailed, nil) || s.Matches(EventDeploymentDrift, &prod) {
		t.Fatal("filter")
	}
	s.Disabled = true
	if s.Matches(EventDeploymentCreated, &prod) {
		t.Fatal("disabled")
	}
}
