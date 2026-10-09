package docsource

import (
	"context"

	"svc-registry/internal/forge"
	"svc-registry/internal/knowledge"
)

type Factory struct {
	Roots        []string
	Forges       forge.ClientFactory
	MaxFileBytes int64
	MaxBranches  int
}

func (f *Factory) Open(_ context.Context, s knowledge.Source, settings knowledge.Settings, token string) (knowledge.Reader, error) {
	switch s.Kind {
	case knowledge.SourceLocalDir:
		return &localDir{path: s.Path, roots: f.Roots, settings: settings, maxFile: f.MaxFileBytes}, nil
	case knowledge.SourceLocalGit:
		return &localGit{path: s.Path, roots: f.Roots, settings: settings, maxFile: f.MaxFileBytes, workingTree: s.WorkingTree,
			includeIgnored: s.IncludeIgnored}, nil
	case knowledge.SourceRemote:
		full := s.FullPath()
		client, err := f.Forges.New(forge.Endpoint{Kind: forge.Kind(s.Forge), APIURL: s.APIURL, Owner: owner(full), Token: token})
		if err != nil {
			return nil, err
		}
		return &remote{client: client, fullPath: full, settings: settings, maxBranches: f.MaxBranches}, nil
	}
	return nil, knowledge.InvalidSource
}
