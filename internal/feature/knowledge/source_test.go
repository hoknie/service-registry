package knowledge

import (
	"errors"
	"testing"
)

func ptr(s string) *string { return &s }

func TestValidateRemoteSources(t *testing.T) {
	for _, c := range []struct {
		in       SourceInput
		url, api string
	}{
		{SourceInput{Kind: "remote", Forge: "gitlab", URL: "https://gitlab.example.com/platform/api/"}, "https://gitlab.example.com/platform/api", "https://gitlab.example.com/api/v4"},
		{SourceInput{Kind: "remote", Forge: "github", URL: "https://github.com/acme/api.git"}, "https://github.com/acme/api", "https://api.github.com"},
		{SourceInput{Kind: "remote", Forge: "github", URL: "https://ghe.example/acme/api"}, "https://ghe.example/acme/api", "https://ghe.example/api/v3"},
		{SourceInput{Kind: "remote", Forge: "forgejo", URL: "http://localhost:3000/acme/api"}, "http://localhost:3000/acme/api", "http://localhost:3000/api/v1"},
		{SourceInput{Kind: "remote", Forge: "gitea", URL: "https://git.example/a/b", APIURL: "https://api.git.example/v1/"}, "https://git.example/a/b", "https://api.git.example/v1"},
	} {
		v, err := ValidateSource(c.in)
		if err != nil || v.URL != c.url || v.APIURL != c.api || !v.Keep {
			t.Errorf("%+v: %+v %v", c.in, v, err)
		}
	}
	v, _ := ValidateSource(SourceInput{Kind: "remote", Forge: "gitlab", URL: "https://g.example/a/b/c"})
	if v.FullPath() != "a/b/c" {
		t.Fatal(v.FullPath())
	}
	v, err := ValidateSource(SourceInput{Kind: "remote", Forge: "github", URL: "https://github.com/a/b", HasCredential: true, Token: ptr("tok")})
	if err != nil || *v.Token != "tok" || v.Keep {
		t.Fatal(v, err)
	}
	v, err = ValidateSource(SourceInput{Kind: "remote", Forge: "github", URL: "https://github.com/a/b", HasCredential: true, NoCredentials: true})
	if err != nil || v.Keep || v.Token != nil {
		t.Fatal(v, err)
	}
	for _, bad := range []SourceInput{
		{Kind: "remote", Forge: "bitbucket", URL: "https://bitbucket.org/a/b"},
		{Kind: "remote", Forge: "github", URL: "http://github.com/a/b"},
		{Kind: "remote", Forge: "github", URL: "https://github.com/a"},
		{Kind: "remote", Forge: "github", URL: "https://user:pw@github.com/a/b"},
		{Kind: "remote", Forge: "github", URL: "https://github.com/a/b?x=1"},
		{Kind: "remote", Forge: "github", URL: "https://github.com/a/b", Path: "/srv"},
		{Kind: "remote", Forge: "github", URL: "https://github.com/a/b", HasCredential: true, Token: ptr("a b")},
		{Kind: "remote", Forge: "github", URL: "https://github.com/a/b", HasCredential: true, Reference: ptr("vault:x")},
		{Kind: "svn", Path: "/srv"},
	} {
		if _, err := ValidateSource(bad); !errors.Is(err, InvalidSource) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
}

func TestValidateLocalSources(t *testing.T) {
	v, err := ValidateSource(SourceInput{Kind: "local_git", Path: " /srv/repos/api "})
	if err != nil || v.Path != "/srv/repos/api" || !v.IsLocal() {
		t.Fatal(v, err)
	}
	for _, bad := range []SourceInput{{Kind: "local_dir", Path: "docs"}, {Kind: "local_dir"}, {Kind: "local_dir", Path: "/srv", URL: "https://x/a/b"}} {
		if _, err := ValidateSource(bad); !errors.Is(err, InvalidSource) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	if _, err := ValidateSource(SourceInput{Kind: "local_dir", Path: "/srv/docs/../etc"}); !errors.Is(err, InvalidPath) {
		t.Fatal(err)
	}
}
