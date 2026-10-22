package forgefake

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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
