package forgefake

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type Repo struct {
	ID            int64
	Path          string
	Name          string
	Description   string
	Topics        []string
	Archived      bool
	Fork          bool
	Private       bool
	Stars         int
	License       string
	DefaultBranch string
	Pushed        time.Time
	Readme        string
	Languages     map[string]int
	Branches      []Branch
	Files         map[string]string
	TreeTruncated bool
}

func BlobSHA(content string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(content), content))))
}

func (r *Repo) tree() []string {
	paths := make([]string, 0, len(r.Files))
	for p := range r.Files {
		paths = append(paths, p)
	}
	for i := 1; i < len(paths); i++ {
		for j := i; j > 0 && paths[j] < paths[j-1]; j-- {
			paths[j], paths[j-1] = paths[j-1], paths[j]
		}
	}
	return paths
}

func (r *Repo) blob(sha string) (string, bool) {
	for _, c := range r.Files {
		if BlobSHA(c) == sha {
			return c, true
		}
	}
	return "", false
}

type Branch struct {
	Name      string
	SHA       string
	Protected bool
	Date      time.Time
}

func (r *Repo) branches() []Branch {
	if len(r.Branches) > 0 {
		return r.Branches
	}
	return []Branch{{Name: r.DefaultBranch, SHA: fmt.Sprintf("%040x", r.ID), Date: r.Pushed}}
}

type Hook struct {
	ID     int64
	URL    string
	Secret string
}

type Fake struct {
	Kind        string
	Token       string
	Owner       string
	OwnerIsUser bool
	PageSize    int
	Anonymous   bool

	srv         *httptest.Server
	mu          sync.Mutex
	repos       map[int64]*Repo
	hooks       map[int64]Hook
	nextHook    int64
	limitedTill time.Time
	hookFail    int
	delay       time.Duration
	calls       map[string]int
}

func (f *Fake) Delay(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delay = d
}

func Start(t testing.TB, kind, token, owner string) *Fake {
	f := &Fake{Kind: kind, Token: token, Owner: owner, PageSize: 2, repos: map[int64]*Repo{}, hooks: map[int64]Hook{}, calls: map[string]int{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *Fake) APIURL() string { return f.srv.URL }

func (f *Fake) Put(r Repo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.DefaultBranch == "" {
		r.DefaultBranch = "main"
	}
	if r.Name == "" {
		r.Name = r.Path[strings.LastIndexByte(r.Path, '/')+1:]
	}
	if r.Pushed.IsZero() {
		r.Pushed = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	cp := r
	f.repos[r.ID] = &cp
}

func (f *Fake) SetFiles(id int64, files map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repos[id].Files = files
}

func (f *Fake) SetBranches(id int64, branches []Branch) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repos[id].Branches = branches
}

func (f *Fake) Delete(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.repos, id)
}

func (f *Fake) RateLimitUntil(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.limitedTill = t
}

func (f *Fake) FailHooks(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hookFail = status
}

func (f *Fake) Hooks() []Hook {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Hook
	for _, h := range f.hooks {
		out = append(out, h)
	}
	return out
}

func (f *Fake) Calls(kind string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[kind]
}

func (f *Fake) authorized(r *http.Request) bool {
	if f.Anonymous && r.Header.Get("Authorization") == "" && r.Header.Get("PRIVATE-TOKEN") == "" {
		return true
	}
	switch f.Kind {
	case "github":
		return r.Header.Get("Authorization") == "Bearer "+f.Token
	case "gitlab":
		return r.Header.Get("PRIVATE-TOKEN") == f.Token
	default:
		return r.Header.Get("Authorization") == "token "+f.Token
	}
}

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

func (f *Fake) github(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	ownerOK := func(kind, name string) bool {
		return strings.EqualFold(name, f.Owner) && (kind == "users") == f.OwnerIsUser
	}
	switch {
	case len(parts) == 2 && (parts[0] == "orgs" || parts[0] == "users") && r.Method == "GET":
		if !ownerOK(parts[0], parts[1]) {
			notFound(w)
			return
		}
		writeJSON(w, 200, map[string]any{"login": f.Owner, "id": 1})
	case len(parts) == 3 && (parts[0] == "orgs" || parts[0] == "users") && parts[2] == "repos":
		if !ownerOK(parts[0], parts[1]) {
			notFound(w)
			return
		}
		f.calls["list"]++
		items, next := page(f.ownerRepos(), r, f.PageSize, "page")
		if next != 0 {
			u := *r.URL
			q := u.Query()
			q.Set("page", strconv.Itoa(next))
			u.RawQuery = q.Encode()
			w.Header().Set("Link", fmt.Sprintf(`<%s%s>; rel="next"`, f.srv.URL, u.RequestURI()))
		}
		out := make([]map[string]any, 0, len(items))
		for _, x := range items {
			out = append(out, githubJSON(x))
		}
		writeJSON(w, 200, out)
	case len(parts) == 3 && parts[0] == "repos" && r.Method == "GET":
		f.calls["repo"]++
		x := f.byPath(parts[1] + "/" + parts[2])
		if x == nil {
			notFound(w)
			return
		}
		writeJSON(w, 200, githubJSON(x))
	case len(parts) == 4 && parts[0] == "repos" && parts[3] == "readme":
		f.calls["readme"]++
		x := f.byPath(parts[1] + "/" + parts[2])
		if x == nil || x.Readme == "" {
			notFound(w)
			return
		}
		writeJSON(w, 200, map[string]any{"type": "file", "name": "README.md", "path": "README.md", "encoding": "base64",
			"content": base64.StdEncoding.EncodeToString([]byte(x.Readme))})
	case len(parts) == 4 && parts[0] == "repos" && parts[3] == "branches":
		f.calls["branches"]++
		x := f.byPath(parts[1] + "/" + parts[2])
		if x == nil {
			notFound(w)
			return
		}
		size, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		items, next := page(x.branches(), r, max(size, 1), "page")
		if next != 0 {
			u := *r.URL
			q := u.Query()
			q.Set("page", strconv.Itoa(next))
			u.RawQuery = q.Encode()
			w.Header().Set("Link", fmt.Sprintf(`<%s%s>; rel="next"`, f.srv.URL, u.RequestURI()))
		}
		out := make([]map[string]any, 0, len(items))
		for _, b := range items {
			out = append(out, map[string]any{"name": b.Name, "protected": b.Protected, "commit": map[string]any{"sha": b.SHA}})
		}
		writeJSON(w, 200, out)
	case len(parts) == 4 && parts[0] == "repos" && parts[3] == "languages":
		f.calls["languages"]++
		x := f.byPath(parts[1] + "/" + parts[2])
		if x == nil {
			notFound(w)
			return
		}
		writeJSON(w, 200, x.Languages)
	case len(parts) == 6 && parts[0] == "repos" && parts[3] == "git" && parts[4] == "trees":
		f.calls["tree"]++
		x := f.byPath(parts[1] + "/" + parts[2])
		if x == nil {
			notFound(w)
			return
		}
		entries := []map[string]any{}
		for _, p := range x.tree() {
			entries = append(entries, map[string]any{"path": p, "mode": "100644", "type": "blob", "sha": BlobSHA(x.Files[p]), "size": len(x.Files[p])})
		}
		writeJSON(w, 200, map[string]any{"sha": parts[5], "tree": entries, "truncated": x.TreeTruncated})
	case len(parts) == 6 && parts[0] == "repos" && parts[3] == "git" && parts[4] == "blobs":
		f.calls["blob"]++
		x := f.byPath(parts[1] + "/" + parts[2])
		content, ok := "", false
		if x != nil {
			content, ok = x.blob(parts[5])
		}
		if !ok {
			notFound(w)
			return
		}
		_, _ = w.Write([]byte(content))
	case len(parts) == 3 && parts[0] == "orgs" && parts[2] == "hooks" && r.Method == "POST":
		f.hook(w, r, func(body map[string]any) Hook {
			cfg, _ := body["config"].(map[string]any)
			return Hook{URL: str(cfg, "url"), Secret: str(cfg, "secret")}
		})
	case len(parts) == 4 && parts[0] == "orgs" && parts[2] == "hooks" && r.Method == "DELETE":
		f.deleteHook(w, parts[3])
	default:
		notFound(w)
	}
}

func githubJSON(x *Repo) map[string]any {
	vis := "public"
	if x.Private {
		vis = "private"
	}
	m := map[string]any{
		"id": x.ID, "name": x.Name, "full_name": x.Path, "description": x.Description, "topics": x.Topics,
		"html_url": "https://github.example/" + x.Path, "default_branch": x.DefaultBranch, "archived": x.Archived,
		"fork": x.Fork, "private": x.Private, "visibility": vis, "stargazers_count": x.Stars,
		"pushed_at": x.Pushed.Format(time.RFC3339), "updated_at": x.Pushed.Format(time.RFC3339),
	}
	if x.License != "" {
		m["license"] = map[string]any{"spdx_id": x.License, "name": x.License}
	}
	return m
}

func (f *Fake) gitlab(w http.ResponseWriter, r *http.Request) {
	rest, ok := strings.CutPrefix(r.URL.EscapedPath(), "/api/v4/")
	if !ok {
		notFound(w)
		return
	}
	parts := strings.Split(rest, "/")
	unesc := func(s string) string { v, _ := url.PathUnescape(s); return v }
	switch {
	case len(parts) == 2 && parts[0] == "groups" && r.Method == "GET":
		if !strings.EqualFold(unesc(parts[1]), f.Owner) {
			notFound(w)
			return
		}
		writeJSON(w, 200, map[string]any{"id": 1, "full_path": f.Owner})
	case len(parts) == 3 && parts[0] == "groups" && parts[2] == "projects":
		if !strings.EqualFold(unesc(parts[1]), f.Owner) {
			notFound(w)
			return
		}
		f.calls["list"]++
		items, next := page(f.ownerRepos(), r, f.PageSize, "page")
		if next != 0 {
			w.Header().Set("X-Next-Page", strconv.Itoa(next))
		}
		w.Header().Set("X-Page", r.URL.Query().Get("page"))
		out := make([]map[string]any, 0, len(items))
		for _, x := range items {
			out = append(out, gitlabJSON(x))
		}
		writeJSON(w, 200, out)
	case len(parts) >= 2 && parts[0] == "projects":
		id, err := strconv.ParseInt(parts[1], 10, 64)
		x := f.repos[id]
		if err != nil {
			x = f.byPath(unesc(parts[1]))
		}
		if x == nil {
			notFound(w)
			return
		}
		switch {
		case len(parts) == 2:
			f.calls["repo"]++
			m := gitlabJSON(x)
			if x.License != "" {
				m["license"] = map[string]any{"key": strings.ToLower(x.License), "name": x.License}
			}
			writeJSON(w, 200, m)
		case len(parts) == 4 && parts[2] == "repository" && parts[3] == "branches":
			f.calls["branches"]++
			size, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
			items, next := page(x.branches(), r, max(size, 1), "page")
			if next != 0 {
				w.Header().Set("X-Next-Page", strconv.Itoa(next))
			}
			out := make([]map[string]any, 0, len(items))
			for _, b := range items {
				out = append(out, map[string]any{"name": b.Name, "protected": b.Protected, "default": b.Name == x.DefaultBranch,
					"commit": map[string]any{"id": b.SHA, "committed_date": b.Date.Format(time.RFC3339)}})
			}
			writeJSON(w, 200, out)
		case len(parts) == 3 && parts[2] == "languages":
			f.calls["languages"]++
			total := 0
			for _, v := range x.Languages {
				total += v
			}
			pct := map[string]float64{}
			for k, v := range x.Languages {
				pct[k] = float64(v) * 100 / float64(max(total, 1))
			}
			writeJSON(w, 200, pct)
		case len(parts) == 4 && parts[2] == "repository" && parts[3] == "tree":
			f.calls["tree"]++
			entries := []map[string]any{}
			for _, p := range x.tree() {
				entries = append(entries, map[string]any{"id": BlobSHA(x.Files[p]), "name": p[strings.LastIndexByte(p, '/')+1:],
					"type": "blob", "path": p, "mode": "100644"})
			}
			size, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
			items, next := page(entries, r, max(min(size, f.PageSize), 1), "page")
			if next != 0 {
				w.Header().Set("X-Next-Page", strconv.Itoa(next))
			}
			writeJSON(w, 200, items)
		case len(parts) == 6 && parts[2] == "repository" && parts[3] == "blobs" && parts[5] == "raw":
			f.calls["blob"]++
			content, ok := x.blob(parts[4])
			if !ok {
				notFound(w)
				return
			}
			_, _ = w.Write([]byte(content))
		case len(parts) == 6 && parts[2] == "repository" && parts[3] == "files" && parts[5] == "raw":
			f.calls["readme"]++
			if x.Readme == "" || unesc(parts[4]) != "README.md" {
				notFound(w)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(x.Readme))
		default:
			notFound(w)
		}
	case len(parts) == 3 && parts[0] == "groups" && parts[2] == "hooks" && r.Method == "POST":
		f.hook(w, r, func(body map[string]any) Hook { return Hook{URL: str(body, "url"), Secret: str(body, "token")} })
	case len(parts) == 4 && parts[0] == "groups" && parts[2] == "hooks" && r.Method == "DELETE":
		f.deleteHook(w, parts[3])
	default:
		notFound(w)
	}
}

func gitlabJSON(x *Repo) map[string]any {
	vis := "public"
	if x.Private {
		vis = "private"
	}
	m := map[string]any{
		"id": x.ID, "name": x.Name, "path_with_namespace": x.Path, "description": x.Description, "topics": x.Topics,
		"web_url": "https://gitlab.example/" + x.Path, "default_branch": x.DefaultBranch, "archived": x.Archived,
		"visibility": vis, "star_count": x.Stars,
		"last_activity_at": x.Pushed.Format(time.RFC3339), "updated_at": x.Pushed.Format(time.RFC3339),
	}
	if x.Fork {
		m["forked_from_project"] = map[string]any{"id": 999}
	}
	if x.Readme != "" {
		m["readme_url"] = "https://gitlab.example/" + x.Path + "/-/blob/" + x.DefaultBranch + "/README.md"
	}
	return m
}

func giteaJSON(x *Repo) map[string]any {
	m := map[string]any{
		"id": x.ID, "name": x.Name, "full_name": x.Path, "description": x.Description, "topics": x.Topics,
		"html_url": "https://gitea.example/" + x.Path, "default_branch": x.DefaultBranch, "archived": x.Archived,
		"fork": x.Fork, "private": x.Private, "stars_count": x.Stars, "updated_at": x.Pushed.Format(time.RFC3339),
	}
	if x.License != "" {
		m["licenses"] = []string{x.License}
	}
	return m
}

func (f *Fake) gitea(w http.ResponseWriter, r *http.Request) {
	rest, ok := strings.CutPrefix(r.URL.Path, "/api/v1/")
	if !ok {
		notFound(w)
		return
	}
	parts := strings.Split(rest, "/")
	ownerOK := func(kind, name string) bool {
		return strings.EqualFold(name, f.Owner) && (kind == "users") == f.OwnerIsUser
	}
	switch {
	case len(parts) == 2 && (parts[0] == "orgs" || parts[0] == "users") && r.Method == "GET":
		if !ownerOK(parts[0], parts[1]) {
			notFound(w)
			return
		}
		writeJSON(w, 200, map[string]any{"id": 1, "username": f.Owner, "login": f.Owner})
	case len(parts) == 3 && (parts[0] == "orgs" || parts[0] == "users") && parts[2] == "repos":
		if !ownerOK(parts[0], parts[1]) {
			notFound(w)
			return
		}
		f.calls["list"]++
		size, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if size <= 0 {
			size = f.PageSize
		}
		items, _ := page(f.ownerRepos(), r, size, "page")
		out := make([]map[string]any, 0, len(items))
		for _, x := range items {
			out = append(out, giteaJSON(x))
		}
		writeJSON(w, 200, out)
	case len(parts) == 3 && parts[0] == "repos" && r.Method == "GET":
		f.calls["repo"]++
		x := f.byPath(parts[1] + "/" + parts[2])
		if x == nil {
			notFound(w)
			return
		}
		writeJSON(w, 200, giteaJSON(x))
	case len(parts) >= 4 && parts[0] == "repos":
		x := f.byPath(parts[1] + "/" + parts[2])
		if x == nil {
			notFound(w)
			return
		}
		switch {
		case parts[3] == "contents" && (len(parts) == 4 || (len(parts) == 5 && parts[4] == "")):
			var entries []map[string]any
			if x.Readme != "" {
				entries = append(entries, map[string]any{"name": "README.md", "path": "README.md", "type": "file"})
			}
			entries = append(entries, map[string]any{"name": "main.go", "path": "main.go", "type": "file"})
			writeJSON(w, 200, entries)
		case parts[3] == "raw" && len(parts) == 5:
			f.calls["readme"]++
			if x.Readme == "" || parts[4] != "README.md" {
				notFound(w)
				return
			}
			_, _ = w.Write([]byte(x.Readme))
		case parts[3] == "languages":
			f.calls["languages"]++
			writeJSON(w, 200, x.Languages)
		case len(parts) == 6 && parts[3] == "git" && parts[4] == "trees":
			f.calls["tree"]++
			entries := []map[string]any{}
			for _, p := range x.tree() {
				entries = append(entries, map[string]any{"path": p, "mode": "100644", "type": "blob", "sha": BlobSHA(x.Files[p]), "size": len(x.Files[p])})
			}
			size, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			pageNo, _ := strconv.Atoi(r.URL.Query().Get("page"))
			items, _ := page(entries, r, max(min(size, f.PageSize), 1), "page")
			writeJSON(w, 200, map[string]any{"sha": parts[5], "tree": items, "truncated": x.TreeTruncated,
				"page": max(pageNo, 1), "total_count": len(entries)})
		case len(parts) == 6 && parts[3] == "git" && parts[4] == "blobs":
			f.calls["blob"]++
			content, ok := x.blob(parts[5])
			if !ok {
				notFound(w)
				return
			}
			writeJSON(w, 200, map[string]any{"content": base64.StdEncoding.EncodeToString([]byte(content)), "encoding": "base64",
				"sha": parts[5], "size": len(content)})
		case parts[3] == "branches" && len(parts) == 4:
			f.calls["branches"]++
			size, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			items, _ := page(x.branches(), r, max(size, 1), "page")
			out := make([]map[string]any, 0, len(items))
			for _, b := range items {
				out = append(out, map[string]any{"name": b.Name, "protected": b.Protected,
					"commit": map[string]any{"id": b.SHA, "timestamp": b.Date.Format(time.RFC3339)}})
			}
			writeJSON(w, 200, out)
		default:
			notFound(w)
		}
	case len(parts) == 3 && parts[0] == "orgs" && parts[2] == "hooks" && r.Method == "POST":
		f.hook(w, r, func(body map[string]any) Hook {
			cfg, _ := body["config"].(map[string]any)
			return Hook{URL: str(cfg, "url"), Secret: str(cfg, "secret")}
		})
	case len(parts) == 4 && parts[0] == "orgs" && parts[2] == "hooks" && r.Method == "DELETE":
		f.deleteHook(w, parts[3])
	default:
		notFound(w)
	}
}
