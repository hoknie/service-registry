package apperr

import "testing"

func TestPublicCodesAndMessages(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err        *Error
		code, text string
	}{
		{&Error{Kind: NotFound}, "not_found", "not found"},
		{&Error{Kind: Unavailable, What: "database"}, "service_unavailable", "database is unavailable"},
		{&Error{Kind: Unavailable, What: "search", Code: "search.unavailable"}, "search.unavailable", "search is unavailable"},
		{&Error{Kind: Validation, Code: "validation.invalid_email", Message: "bad email"}, "validation.invalid_email", "bad email"},
		{&Error{Kind: Conflict, Code: "conflict.email_taken", Message: "taken"}, "conflict.email_taken", "taken"},
		{&Error{Kind: UpstreamRateLimited, Code: "forge.rate_limited", Message: "slow down"}, "forge.rate_limited", "slow down"},
		{&Error{Kind: BadGateway, Code: "forge.upstream", Message: "boom"}, "forge.upstream", "boom"},
		{&Error{Kind: Unprocessable, Code: "ingest.invalid_event", Message: "bad"}, "ingest.invalid_event", "bad"},
		{&Error{Kind: Internal, Detail: "secret detail"}, "internal", "internal server error"},
		{&Error{Kind: Kind(999)}, "internal", "internal server error"},
	}
	for _, c := range cases {
		code, text := c.err.Public()
		if code != c.code || text != c.text {
			t.Errorf("%v: got %q %q, want %q %q", c.err.Kind, code, text, c.code, c.text)
		}
	}
	for k, want := range fixedCodes {
		e := &Error{Kind: k}
		if code, text := e.Public(); code != want || text != e.Error() {
			t.Errorf("%v: got %q %q", k, code, text)
		}
	}
}

func TestEveryKindHasAPublicCode(t *testing.T) {
	t.Parallel()
	for k := NotFound; k <= InvalidWebhookSignature; k++ {
		if k == Internal {
			continue
		}
		if code, _ := (&Error{Kind: k, Code: "x.y"}).Public(); code == "internal" {
			t.Errorf("kind %d has no public code", k)
		}
	}
}
