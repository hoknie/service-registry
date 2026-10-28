package hostingfake

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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
		case parts[3] == "archive" && len(parts) == 5:
			f.archive(w, x)
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
		case parts[3] == "tags" && len(parts) == 4:
			f.calls["tags"]++
			size, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			items, _ := page(x.Tags, r, max(size, 1), "page")
			out := make([]map[string]any, 0, len(items))
			for _, t := range items {
				out = append(out, map[string]any{"name": t.Name, "commit": map[string]any{"sha": t.SHA}})
			}
			writeJSON(w, 200, out)
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
