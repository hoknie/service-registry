package forgeclient

import (
	"net/http"

	domain "svc-registry/internal/forge"
)

type Factory struct{ http *http.Client }

func NewFactory(httpClient *http.Client) *Factory { return &Factory{http: httpClient} }

func (f *Factory) New(e domain.Endpoint) (domain.Client, error) {
	switch e.Kind {
	case domain.KindGithub:
		return newGithub(f.http, e)
	case domain.KindGitlab:
		return newGitlab(f.http, e)
	case domain.KindGitea, domain.KindForgejo:
		return newGitea(f.http, e)
	}
	return nil, &domain.InternalError{Detail: "unknown forge kind " + string(e.Kind)}
}

const pageSize = 50
