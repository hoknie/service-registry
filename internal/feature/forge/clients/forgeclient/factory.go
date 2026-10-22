package forgeclient

import (
	"net/http"

	forgeports "svc-registry/internal/feature/forge/ports"
	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/apperr"
)

type FactoryClient struct{ http *http.Client }

func NewFactoryClient(httpClient *http.Client) *FactoryClient {
	return &FactoryClient{http: httpClient}
}

func (f *FactoryClient) New(e forgetypes.Endpoint) (forgeports.ClientPort, error) {
	switch e.Kind {
	case forgetypes.KindGithub:
		return newGithub(f.http, e)
	case forgetypes.KindGitlab:
		return newGitlab(f.http, e)
	case forgetypes.KindGitea, forgetypes.KindForgejo:
		return newGitea(f.http, e)
	}
	return nil, apperr.InternalDetail("unknown forge kind " + string(e.Kind))
}

const pageSize = 50
