package embeddings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"svc-registry/internal/feature/knowledge"
)

type handler func(w http.ResponseWriter, r *http.Request, input []string, call int)

func server(t *testing.T, h handler) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1))
		var body request
		if r.URL.Path != "/v1/embeddings" || r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("%s %s %q", r.Method, r.URL.Path, r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("body: %v", err)
		}
		h(w, r, body.Input, n)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func vectors(w http.ResponseWriter, input []string, dims int, reverse bool) {
	type item struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	}
	data := make([]item, len(input))
	for i := range input {
		v := make([]float64, dims)
		v[0] = float64(len(input[i]))
		data[i] = item{Index: i, Embedding: v}
	}
	if reverse {
		for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
			data[i], data[j] = data[j], data[i]
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func TestBatchesHeadersAndOrder(t *testing.T) {
	var batches [][]string
	srv, calls := server(t, func(w http.ResponseWriter, r *http.Request, input []string, _ int) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		batches = append(batches, input)
		vectors(w, input, 4, true)
	})
	c := New(srv.URL+"/v1/", "sk-test", "nomic", 4, 2, http.DefaultClient)
	got, err := c.Embed(context.Background(), []string{"a", "bb", "ccc"})
	if err != nil || len(got) != 3 || got[0][0] != 1 || got[1][0] != 2 || got[2][0] != 3 {
		t.Fatalf("%v %v", err, got)
	}
	if calls.Load() != 2 || len(batches[0]) != 2 || len(batches[1]) != 1 {
		t.Fatalf("%d %v", calls.Load(), batches)
	}
}

func TestNoKeySendsPlaceholder(t *testing.T) {
	srv, _ := server(t, func(w http.ResponseWriter, r *http.Request, input []string, _ int) {
		if r.Header.Get("Authorization") != "Bearer none" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		vectors(w, input, 2, false)
	})
	if _, err := New(srv.URL+"/v1", "", "m", 2, 8, http.DefaultClient).Embed(context.Background(), []string{"x"}); err != nil {
		t.Fatal(err)
	}
}

func TestWrongDimensions(t *testing.T) {
	srv, _ := server(t, func(w http.ResponseWriter, _ *http.Request, input []string, _ int) { vectors(w, input, 384, false) })
	_, err := New(srv.URL+"/v1", "", "m", 768, 8, http.DefaultClient).Embed(context.Background(), []string{"x"})
	if !errors.Is(err, knowledge.ErrEmbeddingsDimensions) || !strings.Contains(err.Error(), "384") || !strings.Contains(err.Error(), "768") {
		t.Fatalf("%v", err)
	}
}

func TestRateLimitIsRetriedOnce(t *testing.T) {
	srv, calls := server(t, func(w http.ResponseWriter, _ *http.Request, input []string, call int) {
		if call == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"slow down"}}`))
			return
		}
		vectors(w, input, 2, false)
	})
	if _, err := New(srv.URL+"/v1", "", "m", 2, 8, http.DefaultClient).Embed(context.Background(), []string{"x"}); err != nil || calls.Load() != 2 {
		t.Fatalf("%v %d", err, calls.Load())
	}
}

func TestServerErrorsGiveTheAPIMessage(t *testing.T) {
	srv, calls := server(t, func(w http.ResponseWriter, _ *http.Request, _ []string, _ int) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"model not loaded"}}`))
	})
	_, err := New(srv.URL+"/v1", "", "m", 2, 8, http.DefaultClient).Embed(context.Background(), []string{"x"})
	if !errors.Is(err, knowledge.ErrEmbeddingsUnavailable) || !strings.Contains(err.Error(), "HTTP 500: model not loaded") || calls.Load() != 2 {
		t.Fatalf("%v %d", err, calls.Load())
	}
}

func TestClientErrorsAreNotRetried(t *testing.T) {
	srv, calls := server(t, func(w http.ResponseWriter, _ *http.Request, _ []string, _ int) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`"Model is not an embedding model"`))
	})
	_, err := New(srv.URL+"/v1", "", "m", 2, 8, http.DefaultClient).Embed(context.Background(), []string{"x"})
	if !errors.Is(err, knowledge.ErrEmbeddingsUnavailable) || !strings.Contains(err.Error(), "Model is not an embedding model") || calls.Load() != 1 {
		t.Fatalf("%v %d", err, calls.Load())
	}
}

func TestInvalidJSON(t *testing.T) {
	srv, _ := server(t, func(w http.ResponseWriter, _ *http.Request, _ []string, _ int) { _, _ = w.Write([]byte("<html>")) })
	_, err := New(srv.URL+"/v1", "", "m", 2, 8, http.DefaultClient).Embed(context.Background(), []string{"x"})
	if !errors.Is(err, knowledge.ErrEmbeddingsUnavailable) || !strings.Contains(err.Error(), "invalid response") {
		t.Fatalf("%v", err)
	}
}

func TestCanceledContextStopsTheRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	srv, calls := server(t, func(w http.ResponseWriter, _ *http.Request, _ []string, _ int) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusServiceUnavailable)
		cancel()
	})
	start := time.Now()
	_, err := New(srv.URL+"/v1", "", "m", 2, 8, http.DefaultClient).Embed(ctx, []string{"x"})
	if !errors.Is(err, knowledge.ErrEmbeddingsUnavailable) || calls.Load() != 1 || time.Since(start) > 2*time.Second {
		t.Fatalf("%v %d %s", err, calls.Load(), time.Since(start))
	}
}
