package forgeclient

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"svc-registry/internal/feature/catalog"
	domain "svc-registry/internal/feature/forge"
	"svc-registry/internal/testsupport/forgefake"
)

const token = "tok-123"

func seed(f *forgefake.Fake, owner string) {
	f.Put(forgefake.Repo{ID: 1, Path: owner + "/api", Description: "API", Topics: []string{"go"}, License: "MIT", Stars: 3,
		Readme: "# API", Languages: map[string]int{"Go": 750, "Shell": 250}})
	f.Put(forgefake.Repo{ID: 2, Path: owner + "/web", Archived: true})
	f.Put(forgefake.Repo{ID: 3, Path: owner + "/fork", Fork: true, Private: true})
	f.Put(forgefake.Repo{ID: 4, Path: "other/x"})
}

func client(t *testing.T, f *forgefake.Fake, kind domain.Kind, tok string) domain.Client {
	t.Helper()
	c, err := NewFactory(http.DefaultClient).New(domain.Endpoint{Kind: kind, APIURL: f.APIURL(), Owner: f.Owner, Token: tok})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func paths(repos []domain.RemoteRepo) []string {
	var out []string
	for _, r := range repos {
		out = append(out, r.FullPath)
	}
	sort.Strings(out)
	return out
}

func TestEveryKindListsAllPagesAndDetails(t *testing.T) {
	for _, kind := range []domain.Kind{domain.KindGithub, domain.KindGitlab, domain.KindGitea, domain.KindForgejo} {
		t.Run(string(kind), func(t *testing.T) {
			f := forgefake.Start(t, string(kind), token, "acme")
			seed(f, "acme")
			c := client(t, f, kind, token)
			ctx := context.Background()
			if err := c.Check(ctx); err != nil {
				t.Fatal(err)
			}
			repos, err := c.ListRepos(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if got := paths(repos); len(got) != 3 || got[0] != "acme/api" || got[1] != "acme/fork" || got[2] != "acme/web" {
				t.Fatalf("%v", got)
			}
			var api domain.RemoteRepo
			for _, r := range repos {
				switch r.FullPath {
				case "acme/api":
					api = r
				case "acme/web":
					if !r.Archived {
						t.Error("archived")
					}
				case "acme/fork":
					if !r.Fork || r.Visibility != domain.VisibilityPrivate {
						t.Errorf("fork %+v", r)
					}
				}
			}
			if api.ExternalID != "1" || api.Name != "api" || api.Description != "API" || api.Stars != 3 || api.DefaultBranch != "main" || api.ChangedAt() == nil {
				t.Fatalf("%+v", api)
			}
			d, err := c.Details(ctx, api)
			if err != nil {
				t.Fatal(err)
			}
			if d.Readme == nil || *d.Readme != "# API" || d.Languages["Go"] != 75 || d.Languages["Shell"] != 25 {
				t.Fatalf("%+v", d)
			}
			if d.License == nil || (*d.License != "MIT" && *d.License != "mit") {
				t.Fatalf("license %v", d.License)
			}
		})
	}
}

func TestUserOwners(t *testing.T) {
	for _, kind := range []domain.Kind{domain.KindGithub, domain.KindGitea} {
		f := forgefake.Start(t, string(kind), token, "ann")
		f.OwnerIsUser = true
		seed(f, "ann")
		repos, err := client(t, f, kind, token).ListRepos(context.Background())
		if err != nil || len(repos) != 3 {
			t.Fatalf("%s: %d %v", kind, len(repos), err)
		}
	}
}

func TestGiteaAcceptsTheAPIPath(t *testing.T) {
	for _, suffix := range []string{"/api/v1", "/api/v1/", ""} {
		f := forgefake.Start(t, string(domain.KindGitea), token, "acme")
		seed(f, "acme")
		c, err := NewFactory(http.DefaultClient).New(domain.Endpoint{Kind: domain.KindGitea, APIURL: f.APIURL() + suffix, Owner: "acme", Token: token})
		if err != nil {
			t.Fatal(err)
		}
		if repos, err := c.ListRepos(context.Background()); err != nil || len(repos) != 3 {
			t.Fatalf("%q: %d %v", suffix, len(repos), err)
		}
	}
}

func TestErrorsBecomeDomainErrors(t *testing.T) {
	for _, kind := range []domain.Kind{domain.KindGithub, domain.KindGitlab, domain.KindGitea} {
		t.Run(string(kind), func(t *testing.T) {
			f := forgefake.Start(t, string(kind), token, "acme")
			seed(f, "acme")
			ctx := context.Background()
			if err := client(t, f, kind, "wrong").Check(ctx); !errors.Is(err, domain.ErrUnauthorized) {
				t.Errorf("bad token: %v", err)
			}
			f.Owner = "nobody"
			c := client(t, f, kind, token)
			f.Owner = "acme"
			if err := c.Check(ctx); !errors.Is(err, domain.ErrOwnerNotFound) {
				t.Errorf("unknown owner: %v", err)
			}
			reset := time.Now().Add(10 * time.Minute).Truncate(time.Second)
			f.RateLimitUntil(reset)
			_, err := client(t, f, kind, token).ListRepos(ctx)
			var limited *domain.RateLimited
			if !errors.As(err, &limited) {
				t.Fatalf("rate limit: %v", err)
			}
			if !limited.Reset.Equal(reset) {
				t.Errorf("reset %v, want %v", limited.Reset, reset)
			}
		})
	}
}

func TestNetworkFailureIsUpstream(t *testing.T) {
	c, _ := NewFactory(http.DefaultClient).New(domain.Endpoint{Kind: domain.KindGithub, APIURL: "http://127.0.0.1:1", Owner: "x", Token: "t"})
	var up *domain.Upstream
	if err := c.Check(context.Background()); !errors.As(err, &up) || up.Status != 0 {
		t.Fatal(err)
	}
}

func TestHooksAreRegisteredAndDeleted(t *testing.T) {
	for _, kind := range []domain.Kind{domain.KindGithub, domain.KindGitlab, domain.KindGitea} {
		t.Run(string(kind), func(t *testing.T) {
			f := forgefake.Start(t, string(kind), token, "acme")
			c := client(t, f, kind, token)
			ctx := context.Background()
			id, err := c.RegisterHook(ctx, "https://registry.example/api/v1/forge/hooks/x", "s3cret")
			if err != nil {
				t.Fatal(err)
			}
			hooks := f.Hooks()
			if len(hooks) != 1 || hooks[0].URL != "https://registry.example/api/v1/forge/hooks/x" || hooks[0].Secret != "s3cret" {
				t.Fatalf("%+v", hooks)
			}
			if err := c.DeleteHook(ctx, id); err != nil || len(f.Hooks()) != 0 {
				t.Fatalf("delete: %v %v", err, f.Hooks())
			}
			if err := c.DeleteHook(ctx, id); err != nil {
				t.Fatalf("delete again: %v", err)
			}
			f.FailHooks(http.StatusForbidden)
			var up *domain.Upstream
			if _, err := c.RegisterHook(ctx, "https://x", "s"); !errors.As(err, &up) || up.Status != http.StatusForbidden {
				t.Fatalf("refused: %v", err)
			}
		})
	}
}

func TestDeliverySignatures(t *testing.T) {
	f := NewFactory(nil)
	body := []byte(`{"ref":"refs/heads/main"}`)
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	headers := func(kv ...string) func(string) string {
		return func(name string) string {
			for i := 0; i+1 < len(kv); i += 2 {
				if http.CanonicalHeaderKey(kv[i]) == http.CanonicalHeaderKey(name) {
					return kv[i+1]
				}
			}
			return ""
		}
	}
	cases := []struct {
		kind   domain.Kind
		header func(string) string
		want   bool
	}{
		{domain.KindGithub, headers("X-Hub-Signature-256", "sha256="+sig), true},
		{domain.KindGithub, headers("X-Hub-Signature-256", "sha256="+sig[:60]+"0000"), false},
		{domain.KindGithub, headers(), false},
		{domain.KindGitea, headers("X-Gitea-Signature", sig), true},
		{domain.KindForgejo, headers("X-Forgejo-Signature", sig), true},
		{domain.KindGitea, headers("X-Gitea-Signature", "zz"), false},
		{domain.KindGitlab, headers("X-Gitlab-Token", "s3cret"), true},
		{domain.KindGitlab, headers("X-Gitlab-Token", "s3creT"), false},
	}
	for i, c := range cases {
		if got := f.VerifyDelivery(c.kind, "s3cret", c.header, body); got != c.want {
			t.Errorf("%d %s: %v", i, c.kind, got)
		}
	}
	if f.VerifyDelivery(domain.KindGitlab, "", headers("X-Gitlab-Token", ""), body) {
		t.Error("empty secret")
	}
}

func TestReadmePathOfGitlab(t *testing.T) {
	if got := readmePath("https://gitlab.example/acme/api/-/blob/release/1.0/docs/README.md", "release/1.0"); got != "docs/README.md" {
		t.Error(got)
	}
	if got := readmePath("", "main"); got != "" {
		t.Error(got)
	}
}

func TestBranchesOfEveryKind(t *testing.T) {
	for _, kind := range []domain.Kind{domain.KindGithub, domain.KindGitlab, domain.KindGitea} {
		t.Run(string(kind), func(t *testing.T) {
			f := forgefake.Start(t, string(kind), token, "acme")
			day := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
			var branches []forgefake.Branch
			for i := range 120 {
				branches = append(branches, forgefake.Branch{Name: fmt.Sprintf("feature/%03d", i), SHA: fmt.Sprintf("%040X", i+1), Date: day})
			}
			branches = append(branches, forgefake.Branch{Name: "main", SHA: strings.Repeat("a", 40), Protected: true, Date: day})
			f.Put(forgefake.Repo{ID: 1, Path: "acme/api", Branches: branches})
			c := client(t, f, kind, token)
			ctx := context.Background()
			repos, err := c.ListRepos(ctx)
			if err != nil {
				t.Fatal(err)
			}
			all, truncated, err := c.ListBranches(ctx, repos[0], func(string) bool { return true }, 500)
			if err != nil || truncated || len(all) != 121 {
				t.Fatalf("all: %d %v %v", len(all), truncated, err)
			}
			var main catalog.ForgeBranch
			for _, b := range all {
				if b.Name == "main" {
					main = b
				}
			}
			if main.HeadSHA != strings.Repeat("a", 40) || main.Protected == nil || !*main.Protected {
				t.Fatalf("main %+v", main)
			}
			if kind != domain.KindGithub && (main.CommittedAt == nil || !main.CommittedAt.Equal(day)) {
				t.Fatalf("date %v", main.CommittedAt)
			}
			if strings.ToLower(all[0].HeadSHA) != all[0].HeadSHA {
				t.Error("sha not lower case")
			}
			keep := func(n string) bool { return n == "main" || strings.HasPrefix(n, "feature/0") }
			some, truncated, err := c.ListBranches(ctx, repos[0], keep, 50)
			if err != nil || !truncated || len(some) != 50 {
				t.Fatalf("limited: %d %v %v", len(some), truncated, err)
			}
		})
	}
}

func TestEveryKindReadsTreesAndFiles(t *testing.T) {
	files := map[string]string{"README.md": "# API", "openspec/specs/a/spec.md": "## Purpose\nx", "docs/b.md": "bee", "docs/c.md": "sea"}
	for _, kind := range []domain.Kind{domain.KindGithub, domain.KindGitlab, domain.KindGitea, domain.KindForgejo} {
		t.Run(string(kind), func(t *testing.T) {
			f := forgefake.Start(t, string(kind), token, "acme")
			f.Put(forgefake.Repo{ID: 1, Path: "acme/api", Files: files})
			c := client(t, f, kind, token)
			ctx := context.Background()
			repos, err := c.ListRepos(ctx)
			if err != nil {
				t.Fatal(err)
			}
			entries, truncated, err := c.ListTree(ctx, repos[0], "abc123")
			if err != nil || truncated {
				t.Fatal(err, truncated)
			}
			got := map[string]domain.TreeEntry{}
			for _, e := range entries {
				got[e.Path] = e
			}
			if len(got) != len(files) {
				t.Fatalf("%v", entries)
			}
			e := got["docs/b.md"]
			if e.BlobSHA != forgefake.BlobSHA("bee") || (kind != domain.KindGitlab && e.Size != 3) || (kind == domain.KindGitlab && e.Size != -1) {
				t.Fatalf("%+v", e)
			}
			raw, err := c.ReadFile(ctx, repos[0], e, 1024)
			if err != nil || string(raw) != "bee" {
				t.Fatal(string(raw), err)
			}
			if raw, err := c.ReadFile(ctx, repos[0], got["openspec/specs/a/spec.md"], 3); err != nil || len(raw) != 4 {
				t.Fatalf("cut to max+1: %q %v", raw, err)
			}
			if _, err := c.ReadFile(ctx, repos[0], domain.TreeEntry{Path: "x", BlobSHA: forgefake.BlobSHA("nope")}, 10); !errors.Is(err, domain.ErrRepoNotFound) {
				t.Fatal(err)
			}
			f.RateLimitUntil(time.Now().Add(time.Minute))
			if _, _, err := c.ListTree(ctx, repos[0], "abc123"); domain.FailureCode(err) != "forge.rate_limited" {
				t.Fatal(err)
			}
		})
	}
}

func TestGithubReportsATruncatedTree(t *testing.T) {
	f := forgefake.Start(t, "github", token, "acme")
	f.Put(forgefake.Repo{ID: 1, Path: "acme/api", Files: map[string]string{"README.md": "x"}, TreeTruncated: true})
	c := client(t, f, domain.KindGithub, token)
	repos, _ := c.ListRepos(context.Background())
	if _, truncated, err := c.ListTree(context.Background(), repos[0], "abc"); err != nil || !truncated {
		t.Fatal(err, truncated)
	}
}

func TestEveryKindFindsOneRepository(t *testing.T) {
	for _, kind := range []domain.Kind{domain.KindGithub, domain.KindGitlab, domain.KindGitea, domain.KindForgejo} {
		t.Run(string(kind), func(t *testing.T) {
			f := forgefake.Start(t, string(kind), token, "acme")
			f.Put(forgefake.Repo{ID: 1, Path: "acme/api", DefaultBranch: "trunk", Files: map[string]string{"README.md": "x"}})
			c := client(t, f, kind, token)
			ctx := context.Background()
			r, err := c.Repo(ctx, "acme/api")
			if err != nil || r.DefaultBranch != "trunk" || r.FullPath != "acme/api" {
				t.Fatalf("%+v %v", r, err)
			}
			if _, err := c.Repo(ctx, "acme/none"); !errors.Is(err, domain.ErrRepoNotFound) {
				t.Fatal(err)
			}
			if _, _, err := c.ListTree(ctx, domain.RemoteRepo{ExternalID: "acme/api", FullPath: "acme/api"}, "abc"); err != nil {
				t.Fatal(err)
			}
			anon := client(t, f, kind, "")
			if _, err := anon.Repo(ctx, "acme/api"); domain.FailureCode(err) != "forge.unauthorized" {
				t.Fatal(err)
			}
			f.Anonymous = true
			if _, err := anon.Repo(ctx, "acme/api"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
