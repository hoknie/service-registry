package forgeclient

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"testing"

	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/pkg/testing/forgefake"
)

func TestEveryKindServesTheRepositoryArchive(t *testing.T) {
	for _, kind := range []forgetypes.Kind{forgetypes.KindGithub, forgetypes.KindGitlab, forgetypes.KindGitea, forgetypes.KindForgejo} {
		t.Run(string(kind), func(t *testing.T) {
			f := forgefake.Start(t, string(kind), token, "acme")
			f.Put(forgefake.Repo{ID: 1, Path: "acme/skills", Name: "skills", Files: map[string]string{"a/SKILL.md": "x", "b.txt": "yy"}})
			f.Put(forgefake.Repo{ID: 2, Path: "acme/plain", Name: "plain", NoArchive: true})
			c := client(t, f, kind, token)
			ctx := context.Background()
			repo, err := c.Repo(ctx, "acme/skills")
			if err != nil {
				t.Fatal(err)
			}
			body, err := c.Archive(ctx, repo, "0123")
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			gz, err := gzip.NewReader(body)
			if err != nil {
				t.Fatal(err)
			}
			tr := tar.NewReader(gz)
			var names []string
			for {
				h, err := tr.Next()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if h.Typeflag == tar.TypeReg {
					names = append(names, h.Name)
				}
			}
			if len(names) != 2 || names[0] != "skills-0123abc/a/SKILL.md" {
				t.Fatalf("names = %v", names)
			}
			plain, err := c.Repo(ctx, "acme/plain")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.Archive(ctx, plain, "0123"); !errors.Is(err, forgetypes.ErrArchiveUnsupported) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
