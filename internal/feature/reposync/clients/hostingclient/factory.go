package hostingclient

import (
	"net/http"

	reposyncports "svc-registry/internal/feature/reposync/ports"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
)

type FactoryClient struct{ http *http.Client }

func NewFactoryClient(httpClient *http.Client) *FactoryClient {
	return &FactoryClient{http: httpClient}
}

func (f *FactoryClient) New(e reposynctypes.Endpoint) (reposyncports.ClientPort, error) {
	httpClient := f.http
	if e.Timeout > 0 && e.Timeout != f.http.Timeout {
		httpClient = &http.Client{Transport: f.http.Transport, CheckRedirect: f.http.CheckRedirect, Jar: f.http.Jar, Timeout: e.Timeout}
	}
	switch e.Kind {
	case reposynctypes.KindGithub:
		return newGithub(httpClient, e)
	case reposynctypes.KindGitlab:
		return newGitlab(httpClient, e)
	case reposynctypes.KindGitea, reposynctypes.KindForgejo:
		return newGitea(httpClient, e)
	}
	return nil, apperr.InternalDetail("unknown hosting kind " + string(e.Kind))
}

const pageSize = 50
