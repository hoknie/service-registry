package forgefake

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

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
		case len(parts) == 4 && parts[2] == "repository" && parts[3] == "archive.tar.gz":
			f.archive(w, x)
		case len(parts) == 4 && parts[2] == "repository" && parts[3] == "tags":
			f.calls["tags"]++
			size, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
			items, next := page(x.Tags, r, max(size, 1), "page")
			if next != 0 {
				w.Header().Set("X-Next-Page", strconv.Itoa(next))
			}
			out := make([]map[string]any, 0, len(items))
			for _, t := range items {
				out = append(out, map[string]any{"name": t.Name, "commit": map[string]any{"id": t.SHA}})
			}
			writeJSON(w, 200, out)
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
