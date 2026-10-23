package types

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSignMatchesTheStandardWebhooksVector(t *testing.T) {
	t.Parallel()
	got, err := Sign("whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw", "msg_p5jXN8AQM9LWM0D4loKWxJek", 1614265330, []byte(`{"test": 2432232314}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="; got != want {
		t.Fatalf("signature %q, want %q", got, want)
	}
	if _, err := Sign("MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw", "x", 1, nil); err == nil {
		t.Fatal("a key without whsec_ must be refused")
	}
}

func TestNewSigningSecret(t *testing.T) {
	t.Parallel()
	a, err := NewSigningSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewSigningSecret()
	if !strings.HasPrefix(a, SigningPrefix) || len(a) != len(SigningPrefix)+44 || a == b {
		t.Fatalf("secrets %q %q", a, b)
	}
	if len(Fingerprint(a)) != 4 {
		t.Fatalf("fingerprint %q", Fingerprint(a))
	}
}

func TestWebhookRequestHeaders(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	at := time.Unix(1700000000, 0)
	plain, err := WebhookRequest("https://hooks.example/x", id, []byte(`{}`), nil, at, "svc-registry/1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plain.Headers["webhook-signature"]; ok || plain.Headers["webhook-id"] != id.String() ||
		plain.Headers["webhook-timestamp"] != "1700000000" || plain.Headers["Content-Type"] != "application/json" {
		t.Fatalf("headers %v", plain.Headers)
	}
	secret, _ := NewSigningSecret()
	signed, err := WebhookRequest("https://hooks.example/x", id, []byte(`{}`), &secret, at, "svc-registry/1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := Sign(secret, id.String(), 1700000000, []byte(`{}`))
	if signed.Headers["webhook-signature"] != want {
		t.Fatalf("signature %q, want %q", signed.Headers["webhook-signature"], want)
	}
}
