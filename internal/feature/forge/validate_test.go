package forge

import (
	"reflect"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestCreateDefaultsAndNormalization(t *testing.T) {
	s, err := ValidateCreate(CreateConnection{Kind: "github", OwnerPath: " acme-inc "}, 900)
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{Kind: KindGithub, APIURL: "https://api.github.com", OwnerPath: "acme-inc", IntervalSecs: 900,
		NameInclude: []string{}, NameExclude: []string{}, BranchInclude: []string{}}
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("%+v", s)
	}
	g, err := ValidateCreate(CreateConnection{Kind: "gitlab", OwnerPath: "/acme/platform/", APIURL: ptr("https://git.example.com/"), IntervalSecs: ptr(int64(60))}, 900)
	if err != nil || g.OwnerPath != "acme/platform" || g.APIURL != "https://git.example.com" || !g.MirrorSubgroups || g.IntervalSecs != 60 {
		t.Fatalf("%+v %v", g, err)
	}
	f, _ := ValidateCreate(CreateConnection{Kind: "forgejo", OwnerPath: "acme", APIURL: ptr("https://code.example"), MirrorSubgroups: ptr(true)}, 900)
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
		{CreateConnection{Kind: "github", OwnerPath: "a", APIURL: ptr("ftp://x")}, InvalidAPIURL},
		{CreateConnection{Kind: "github", OwnerPath: "a", APIURL: ptr("https://x?q=1")}, InvalidAPIURL},
		{CreateConnection{Kind: "github", OwnerPath: "a/b"}, InvalidOwnerPath},
		{CreateConnection{Kind: "github", OwnerPath: ""}, InvalidOwnerPath},
		{CreateConnection{Kind: "gitlab", OwnerPath: "a//b"}, InvalidOwnerPath},
		{CreateConnection{Kind: "github", OwnerPath: "a b"}, InvalidOwnerPath},
		{CreateConnection{Kind: "github", OwnerPath: "a", NameInclude: ptr([]string{"[x"})}, InvalidNamePatterns},
		{CreateConnection{Kind: "github", OwnerPath: "a", NameExclude: ptr(make([]string, 33))}, InvalidNamePatterns},
		{CreateConnection{Kind: "github", OwnerPath: "a", IntervalSecs: ptr(int64(59))}, InvalidInterval},
		{CreateConnection{Kind: "github", OwnerPath: "a", IntervalSecs: ptr(int64(86401))}, InvalidInterval},
	}
	for _, tt := range tests {
		if _, err := ValidateCreate(tt.in, 900); err != tt.want {
			t.Errorf("%+v: got %v, want %v", tt.in, err, tt.want)
		}
	}
}

func TestUpdateKeepsWhatIsNotSent(t *testing.T) {
	cur := Connection{Kind: KindGitlab, APIURL: "https://gitlab.com", OwnerPath: "acme", MirrorSubgroups: true,
		NameInclude: []string{"svc-*"}, NameExclude: []string{}, IntervalSecs: 900}
	s, err := ValidateUpdate(cur, UpdateConnection{IncludeArchived: ptr(true)})
	if err != nil || !s.IncludeArchived || !s.MirrorSubgroups || s.NameInclude[0] != "svc-*" || s.IntervalSecs != 900 {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestValidRef(t *testing.T) {
	for _, ref := range []string{"env:FORGE_TOKEN", "file:/run/secrets/t"} {
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
