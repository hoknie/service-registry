package types

import (
	"reflect"
	"testing"
)

func TestCreateDefaultsAndNormalization(t *testing.T) {
	s, err := ValidateCreate(CreateConnection{Kind: "github", OwnerPath: " acme-inc "})
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{Kind: KindGithub, APIURL: "https://api.github.com", OwnerPath: "acme-inc",
		NameInclude: []string{}, NameExclude: []string{}, BranchInclude: []string{}}
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("%+v", s)
	}
	g, err := ValidateCreate(CreateConnection{Kind: "gitlab", OwnerPath: "/acme/platform/", APIURL: new("https://git.example.com/"), IntervalSecs: new(int64(60))})
	if err != nil || g.OwnerPath != "acme/platform" || g.APIURL != "https://git.example.com" || !g.MirrorSubgroups || g.IntervalSecs == nil || *g.IntervalSecs != 60 {
		t.Fatalf("%+v %v", g, err)
	}
	f, _ := ValidateCreate(CreateConnection{Kind: "forgejo", OwnerPath: "acme", APIURL: new("https://code.example"), MirrorSubgroups: new(true)})
	if f.MirrorSubgroups {
		t.Error("mirror kept for forgejo")
	}
}

func TestCreateRules(t *testing.T) {
	tests := []struct {
		in   CreateConnection
		want error
	}{
		{CreateConnection{Kind: "svn", OwnerPath: "a"}, InvalidKind},
		{CreateConnection{Kind: "gitea", OwnerPath: "a"}, InvalidAPIURL},
		{CreateConnection{Kind: "github", OwnerPath: "a", APIURL: new("ftp://x")}, InvalidAPIURL},
		{CreateConnection{Kind: "github", OwnerPath: "a", APIURL: new("https://x?q=1")}, InvalidAPIURL},
		{CreateConnection{Kind: "github", OwnerPath: "a/b"}, InvalidOwnerPath},
		{CreateConnection{Kind: "github", OwnerPath: ""}, InvalidOwnerPath},
		{CreateConnection{Kind: "gitlab", OwnerPath: "a//b"}, InvalidOwnerPath},
		{CreateConnection{Kind: "github", OwnerPath: "a b"}, InvalidOwnerPath},
		{CreateConnection{Kind: "github", OwnerPath: "a", NameInclude: new([]string{"[x"})}, InvalidNamePatterns},
		{CreateConnection{Kind: "github", OwnerPath: "a", NameExclude: new(make([]string, 33))}, InvalidNamePatterns},
		{CreateConnection{Kind: "github", OwnerPath: "a", IntervalSecs: new(int64(59))}, InvalidInterval},
		{CreateConnection{Kind: "github", OwnerPath: "a", IntervalSecs: new(int64(86401))}, InvalidInterval},
	}
	for _, tt := range tests {
		if _, err := ValidateCreate(tt.in); err != tt.want {
			t.Errorf("%+v: got %v, want %v", tt.in, err, tt.want)
		}
	}
}

func TestUpdateKeepsWhatIsNotSent(t *testing.T) {
	cur := Connection{Kind: KindGitlab, APIURL: "https://gitlab.com", OwnerPath: "acme", MirrorSubgroups: true,
		NameInclude: []string{"svc-*"}, NameExclude: []string{}, IntervalSecs: new(900)}
	s, err := ValidateUpdate(cur, UpdateConnection{IncludeArchived: new(true)})
	if err != nil || !s.IncludeArchived || !s.MirrorSubgroups || s.NameInclude[0] != "svc-*" || s.IntervalSecs == nil || *s.IntervalSecs != 900 {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestUpdateClearsInterval(t *testing.T) {
	cur := Connection{Kind: KindGithub, APIURL: "https://api.github.com", OwnerPath: "acme", NameInclude: []string{},
		NameExclude: []string{}, IntervalSecs: new(600)}
	s, err := ValidateUpdate(cur, UpdateConnection{ClearInterval: true})
	if err != nil || s.IntervalSecs != nil {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestValidRef(t *testing.T) {
	for _, ref := range []string{"env:REPO_TOKEN", "file:/run/secrets/t"} {
		if !ValidRef(ref) {
			t.Errorf("%q must be valid", ref)
		}
	}
	for _, ref := range []string{"env:1X", "file:relative", "vault:x", ""} {
		if ValidRef(ref) {
			t.Errorf("%q must be invalid", ref)
		}
	}
}
