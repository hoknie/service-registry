package forgefake

import (
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	Tags          []Tag
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

type Tag struct {
	Name string
	SHA  string
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
	f.repos[r.ID] = new(r)
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

func (f *Fake) SetTags(id int64, tags []Tag) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repos[id].Tags = tags
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
