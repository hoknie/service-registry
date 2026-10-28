package forgeclient

import (
	"context"
	"io"
	"net/http"

	"code.gitea.io/sdk/gitea"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	forgetypes "svc-registry/internal/feature/forge/types"
)

func archiveStatus(status int) error {
	switch status {
	case http.StatusNotFound, http.StatusNotImplemented, http.StatusMethodNotAllowed, http.StatusForbidden:
		return forgetypes.ErrArchiveUnsupported
	}
	return nil
}

func (g *githubClient) Archive(ctx context.Context, repo forgetypes.RemoteRepo, commit string) (io.ReadCloser, error) {
	owner, name := splitFull(repo.FullPath)
	req, err := g.c.NewRequest(ctx, http.MethodGet, "repos/"+owner+"/"+name+"/tarball/"+commit, nil)
	if err != nil {
		return nil, network(err)
	}
	resp, err := g.c.BareDo(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
			if e := archiveStatus(resp.StatusCode); e != nil {
				return nil, e
			}
		}
		return nil, githubErr(err, false)
	}
	return resp.Body, nil
}

func (g *gitlabClient) Archive(ctx context.Context, repo forgetypes.RemoteRepo, commit string) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	started := make(chan error, 1)
	go func() {
		w := &firstWrite{w: pw, started: started}
		resp, err := g.c.Repositories.StreamArchive(repo.ExternalID, w,
			&gitlab.ArchiveOptions{Format: new("tar.gz"), SHA: new(commit)}, gitlab.WithContext(ctx))
		if err != nil {
			if resp != nil && archiveStatus(resp.StatusCode) != nil {
				err = forgetypes.ErrArchiveUnsupported
			} else {
				err = gitlabErr(err, resp, false)
			}
		}
		w.done(err)
		pw.CloseWithError(err)
	}()
	if err := <-started; err != nil {
		return nil, err
	}
	return pr, nil
}

type firstWrite struct {
	w       *io.PipeWriter
	started chan error
	sent    bool
}

func (f *firstWrite) Write(p []byte) (int, error) {
	f.done(nil)
	return f.w.Write(p)
}

func (f *firstWrite) done(err error) {
	if !f.sent {
		f.sent = true
		f.started <- err
	}
}

func (g *giteaClient) Archive(ctx context.Context, repo forgetypes.RemoteRepo, commit string) (io.ReadCloser, error) {
	g.c.SetContext(ctx)
	owner, name := splitFull(repo.FullPath)
	body, resp, err := g.c.GetArchiveReader(owner, name, commit, gitea.TarGZArchive)
	if err != nil {
		if resp != nil && archiveStatus(resp.StatusCode) != nil {
			return nil, forgetypes.ErrArchiveUnsupported
		}
		return nil, giteaErr(err, resp, false)
	}
	return body, nil
}
