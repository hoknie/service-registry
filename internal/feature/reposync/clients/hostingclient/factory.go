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
	switch e.Kind {
	case reposynctypes.KindGithub:
		return newGithub(f.http, e)
	case reposynctypes.KindGitlab:
		return newGitlab(f.http, e)
	case reposynctypes.KindGitea, reposynctypes.KindForgejo:
		return newGitea(f.http, e)
	}
	return nil, apperr.InternalDetail("unknown hosting kind " + string(e.Kind))
}

const pageSize = 50
