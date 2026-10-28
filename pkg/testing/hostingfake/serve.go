package hostingfake

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	delay := f.delay
	f.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
		return
	}
	if time.Now().Before(f.limitedTill) {
		reset := strconv.FormatInt(f.limitedTill.Unix(), 10)
		if f.Kind == "github" {
			w.Header().Set("X-RateLimit-Limit", "5000")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", reset)
			writeJSON(w, http.StatusForbidden, map[string]string{"message": "API rate limit exceeded"})
			return
		}
		w.Header().Set("RateLimit-Reset", reset)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"message": "rate limited"})
		return
	}
	switch f.Kind {
	case "github":
		f.github(w, r)
	case "gitlab":
		f.gitlab(w, r)
	default:
		f.gitea(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
}

func (f *Fake) ownerRepos() []*Repo {
	var out []*Repo
	for _, r := range f.repos {
		if strings.HasPrefix(strings.ToLower(r.Path), strings.ToLower(f.Owner)+"/") {
			out = append(out, r)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].ID < out[j-1].ID; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func (f *Fake) byPath(path string) *Repo {
	for _, r := range f.repos {
		if strings.EqualFold(r.Path, path) {
			return r
		}
	}
	return nil
}

func page[T any](items []T, r *http.Request, size int, param string) ([]T, int) {
	p, _ := strconv.Atoi(r.URL.Query().Get(param))
	if p < 1 {
		p = 1
	}
	from := (p - 1) * size
	if from >= len(items) {
		return []T{}, 0
	}
	to := min(from+size, len(items))
	next := 0
	if to < len(items) {
		next = p + 1
	}
	return items[from:to], next
}

func (f *Fake) hook(w http.ResponseWriter, r *http.Request, create func(body map[string]any) Hook) {
	if f.hookFail != 0 {
		writeJSON(w, f.hookFail, map[string]string{"message": "hooks refused"})
		return
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.nextHook++
	h := create(body)
	h.ID = f.nextHook
	f.hooks[h.ID] = h
	writeJSON(w, http.StatusCreated, map[string]any{"id": h.ID, "url": h.URL, "active": true, "type": "gitea", "config": map[string]string{}})
}

func (f *Fake) deleteHook(w http.ResponseWriter, idText string) {
	id, _ := strconv.ParseInt(idText, 10, 64)
	if _, ok := f.hooks[id]; !ok {
		notFound(w)
		return
	}
	delete(f.hooks, id)
	w.WriteHeader(http.StatusNoContent)
}

func str(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}
