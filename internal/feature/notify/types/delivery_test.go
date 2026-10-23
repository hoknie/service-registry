package types

import (
	"testing"
	"time"
)

func TestPauses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		attempts   int
		retryAfter time.Duration
		want       time.Duration
	}{
		{1, 0, 30 * time.Second},
		{2, 0, 2 * time.Minute},
		{5, 0, time.Hour},
		{8, 0, 12 * time.Hour},
		{9, 0, 12 * time.Hour},
		{1, 5 * time.Minute, 5 * time.Minute},
		{1, 3 * time.Hour, time.Hour},
		{6, 2 * time.Hour, 3 * time.Hour},
	} {
		if got := Pause(tc.attempts, tc.retryAfter); got != tc.want {
			t.Errorf("Pause(%d, %v) = %v, want %v", tc.attempts, tc.retryAfter, got, tc.want)
		}
	}
}

func TestDecide(t *testing.T) {
	t.Parallel()
	code := CodeHTTPStatus
	status := 503
	ok := 204
	for name, tc := range map[string]struct {
		r        Response
		attempts int
		want     Status
	}{
		"delivered":       {Response{HTTPStatus: &ok}, 1, StatusDelivered},
		"retry":           {Response{HTTPStatus: &status, ErrorCode: &code, Retryable: true}, 1, StatusPending},
		"last attempt":    {Response{HTTPStatus: &status, ErrorCode: &code, Retryable: true}, 8, StatusFailed},
		"not retryable":   {Response{HTTPStatus: &status, ErrorCode: &code}, 1, StatusFailed},
		"blocked address": {Response{ErrorCode: new(CodeBlocked)}, 1, StatusFailed},
	} {
		if got := Decide(tc.r, tc.attempts, 8).Status; got != tc.want {
			t.Errorf("%s: %s, want %s", name, got, tc.want)
		}
	}
	for code, want := range map[int]bool{404: false, 408: true, 409: false, 429: true, 500: true, 503: true} {
		if Retryable(code) != want {
			t.Errorf("Retryable(%d) != %v", code, want)
		}
	}
}
