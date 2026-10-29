package hostingclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	reposynctypes "svc-registry/internal/feature/reposync/types"
)

func TestEndpointTimeoutReplacesTheClientOne(t *testing.T) {
	t.Parallel()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	t.Cleanup(slow.Close)
	base := &http.Client{Timeout: 100 * time.Millisecond}
	tests := []struct {
		name     string
		timeout  time.Duration
		upstream bool
	}{
		{"client timeout", 0, true},
		{"shorter endpoint timeout", 50 * time.Millisecond, true},
		{"longer endpoint timeout", 5 * time.Second, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := NewFactoryClient(base).New(reposynctypes.Endpoint{Kind: reposynctypes.KindGithub, APIURL: slow.URL, Owner: "acme",
				Token: "t", Timeout: tt.timeout})
			if err != nil {
				t.Fatal(err)
			}
			err = c.Check(context.Background())
			_, upstream := errors.AsType[*reposynctypes.Upstream](err)
			if upstream != tt.upstream {
				t.Fatalf("Check() = %v, upstream %v, want %v", err, upstream, tt.upstream)
			}
		})
	}
}
