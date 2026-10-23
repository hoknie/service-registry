package registry

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	deploytypes "svc-registry/internal/feature/deploy/types"
)

func TestReportsAreCompressedAndAnswersDecoded(t *testing.T) {
	t.Parallel()
	cluster := uuid.Must(uuid.NewV7())
	var got deploytypes.Report
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer svca_x" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/ingest/clusters/" + cluster.String() + "/config":
			_, _ = w.Write([]byte(`{"cluster":{"name":"edge"},"enabled":true,"namespaces":["a"],"rules":[],"interval_secs":30}`))
		case "/api/v1/ingest/clusters/" + cluster.String() + "/reports":
			if r.Header.Get("Content-Encoding") != "gzip" || r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("headers %v", r.Header)
			}
			if r.ContentLength != -1 || len(r.TransferEncoding) != 1 || r.TransferEncoding[0] != "chunked" {
				t.Errorf("the report is not streamed: length %d, encoding %v", r.ContentLength, r.TransferEncoding)
			}
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			_ = json.NewDecoder(zr).Decode(&got)
			_, _ = w.Write([]byte(`{"enabled":true,"interval_secs":60,"accepted":{"workloads":1,"matched":1,"unmatched":0}}`))
		}
	}))
	t.Cleanup(srv.Close)
	c := NewRegistryClient(srv.Client(), srv.URL+"/", cluster, "svca_x", "0.2.0")
	cfg, err := c.Config(context.Background())
	if err != nil || cfg.IntervalSecs != 30 || cfg.Cluster.Name != "edge" {
		t.Fatalf("config %+v %v", cfg, err)
	}
	answer, err := c.Report(context.Background(), deploytypes.Report{AgentVersion: "0.2.0",
		Workloads: []deploytypes.ReportWorkload{{UID: "u1", Kind: deploytypes.KindDeployment, Namespace: "a", Name: "api"}}})
	if err != nil || answer.Accepted.Matched != 1 || answer.IntervalSecs != 60 {
		t.Fatalf("answer %+v %v", answer, err)
	}
	if got.AgentVersion != "0.2.0" || len(got.Workloads) != 1 || got.Workloads[0].UID != "u1" {
		t.Fatalf("sent %+v", got)
	}
	_, err = NewRegistryClient(srv.Client(), srv.URL, cluster, "svca_y", "0.2.0").Config(context.Background())
	if !errors.Is(err, deploytypes.ErrAgentRefused) {
		t.Fatalf("401: %v", err)
	}
}

func TestRetryAfterIsKept(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "17")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":"ingest.rate_limited","message":"slow down"}`))
	}))
	t.Cleanup(srv.Close)
	_, err := NewRegistryClient(srv.Client(), srv.URL, uuid.Nil, "t", "v").Report(context.Background(), deploytypes.Report{})
	re, ok := errors.AsType[*deploytypes.RegistryError](err)
	if !ok || re.Status != 429 || re.Code != "ingest.rate_limited" || re.RetryAfter != 17*time.Second {
		t.Fatalf("%#v", err)
	}
}
