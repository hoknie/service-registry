package responses

import (
	"net/http"
	"testing"

	"svc-registry/internal/platform/apperr"
)

func TestEveryKindHasAStatus(t *testing.T) {
	t.Parallel()
	for k := apperr.NotFound; k <= apperr.InvalidWebhookSignature; k++ {
		if k == apperr.Internal {
			continue
		}
		if _, ok := statuses[k]; !ok {
			t.Errorf("kind %d has no status", k)
		}
	}
}

func TestFromAppKeepsHeadersAndFields(t *testing.T) {
	t.Parallel()
	api := FromApp(&apperr.Error{Kind: apperr.RateLimited, RetryAfterSecs: 7})
	if api.Status != http.StatusTooManyRequests || api.Code != "auth.rate_limited" || api.RetryAfter != 7 {
		t.Fatalf("%+v", api)
	}
	api = FromApp(&apperr.Error{Kind: apperr.InvalidToken})
	if api.Status != http.StatusUnauthorized || api.Code != "auth.invalid_token" || api.Challenge != "Bearer" {
		t.Fatalf("%+v", api)
	}
	fields := []apperr.Field{{Path: "type", Code: "required"}}
	api = FromApp(&apperr.Error{Kind: apperr.Unprocessable, Code: "ingest.invalid_event", Message: "bad", Fields: fields})
	if api.Status != http.StatusUnprocessableEntity || len(api.Fields) != 1 || api.Code != "ingest.invalid_event" {
		t.Fatalf("%+v", api)
	}
	api = FromApp(&apperr.Error{Kind: apperr.Internal, Detail: "x"})
	if api.Status != http.StatusInternalServerError || api.Code != "internal" {
		t.Fatalf("%+v", api)
	}
}
