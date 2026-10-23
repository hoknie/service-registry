package console

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type agentProbes interface {
	Live() bool
	Ready() bool
}

func agentHealthHandler(p agentProbes) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", probe(p.Live))
	mux.HandleFunc("GET /readyz", probe(p.Ready))
	return mux
}

func probe(ok func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if ok() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"unavailable"}` + "\n"))
	}
}

func serveAgentHealth(addr string, p agentProbes) (func(), error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("--health-addr: %w", err)
	}
	srv := &http.Server{Handler: agentHealthHandler(p), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("agent: health server stopped", "error", err)
		}
	}()
	slog.Info("agent: health checks listening", "addr", ln.Addr().String())
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}, nil
}
