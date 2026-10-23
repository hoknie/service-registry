package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	svcapp "svc-registry/internal/app"
	"svc-registry/internal/feature/notify/clients/sender"
	"svc-registry/pkg/outbound"
)

func subscriptionsPath(node string) string { return nodePath(node) + "/notification-subscriptions" }

func deliveriesPath(node string) string { return nodePath(node) + "/notification-deliveries" }

type received struct {
	header http.Header
	body   []byte
	at     time.Time
}

type receiver struct {
	*httptest.Server
	mu       sync.Mutex
	statuses []int
	headers  map[string]string
	got      []received
}

func newReceiver(t testing.TB, statuses ...int) *receiver {
	t.Helper()
	rc := &receiver{statuses: statuses, headers: map[string]string{}}
	rc.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rc.mu.Lock()
		rc.got = append(rc.got, received{header: r.Header.Clone(), body: body, at: time.Now()})
		status := http.StatusNoContent
		if len(rc.statuses) > 0 {
			status, rc.statuses = rc.statuses[0], rc.statuses[1:]
		}
		for k, v := range rc.headers {
			w.Header().Set(k, v)
		}
		rc.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(rc.Close)
	return rc
}

func (rc *receiver) requests() []received {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return append([]received{}, rc.got...)
}

func (rc *receiver) payload(t testing.TB, i int) obj {
	t.Helper()
	got := rc.requests()
	if len(got) <= i {
		t.Fatalf("receiver got %d requests, want more than %d", len(got), i)
	}
	var v obj
	must(t, json.Unmarshal(got[i].body, &v))
	return v
}

func startNotifyApp(t testing.TB, opts []svcapp.Option, extra ...string) *testApp {
	t.Helper()
	loopback := sender.New(outbound.Options{}, 5*time.Second, anyIP{})
	return startAppWith(t, append([]svcapp.Option{svcapp.WithNotifySender(loopback)}, opts...),
		append([]string{"SECRETS_KEYS", secretsKey, "PUBLIC_URL", "https://registry.example"}, extra...)...)
}

func (a *testApp) subscribe(cookie, node, key string, body obj) reply {
	a.t.Helper()
	return a.send("PUT", subscriptionsPath(node)+"/"+key, cookie, body)
}

func (a *testApp) mustSubscribe(cookie, node, key string, body obj) obj {
	a.t.Helper()
	r := a.subscribe(cookie, node, key, body)
	if r.status != 200 {
		a.t.Fatalf("subscribe %s: %d %s", key, r.status, r.body)
	}
	return r.json(a.t)
}

func (a *testApp) hook(cookie, node, url string, events ...string) obj {
	a.t.Helper()
	if len(events) == 0 {
		events = []string{"deployment.created"}
	}
	return obj{"channel": "webhook", "secret_id": a.secret(cookie, node, obj{"value": url}), "event_types": events}
}

func (a *testApp) deliverNow() int {
	a.t.Helper()
	execSQL(a.t, a.db, "UPDATE notification_deliveries SET next_run_at = now() WHERE status = 'pending'")
	ctx := context.Background()
	ids, err := a.services.Notify.Deliveries.ClaimDeliveries(ctx, 100, 120)
	if err != nil {
		a.t.Fatal(err)
	}
	for _, id := range ids {
		if err := a.services.Notify.Deliveries.RunDelivery(ctx, id); err != nil {
			a.t.Fatal(err)
		}
	}
	return len(ids)
}

func (a *testApp) deliveriesOf(cookie, node, query string) []any {
	a.t.Helper()
	r := a.get(deliveriesPath(node)+query, cookie)
	if r.status != 200 {
		a.t.Fatalf("deliveries: %d %s", r.status, r.body)
	}
	return list(a.t, r.json(a.t), "items")
}
