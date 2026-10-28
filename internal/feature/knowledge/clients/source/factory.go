package source

import (
	"context"

	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	reposyncports "svc-registry/internal/feature/reposync/ports"
	reposynctypes "svc-registry/internal/feature/reposync/types"
)

type FactoryClient struct {
	Roots        []string
	Hostings     reposyncports.ClientFactoryPort
	MaxFileBytes int64
	MaxBranches  int
}

func (f *FactoryClient) Open(_ context.Context, s knowledgetypes.Source, settings knowledgetypes.Settings, token string) (knowledgeports.ReaderPort, error) {
	switch s.Kind {
	case knowledgetypes.SourceLocalDir:
		return &localDir{path: s.Path, roots: f.Roots, settings: settings, maxFile: f.MaxFileBytes}, nil
	case knowledgetypes.SourceLocalGit:
		return &localGit{path: s.Path, roots: f.Roots, settings: settings, maxFile: f.MaxFileBytes, workingTree: s.WorkingTree,
			includeIgnored: s.IncludeIgnored}, nil
	case knowledgetypes.SourceRemote:
		full := s.FullPath()
		client, err := f.Hostings.New(reposynctypes.Endpoint{Kind: reposynctypes.Kind(s.Hosting), APIURL: s.APIURL, Owner: owner(full), Token: token})
		if err != nil {
			return nil, err
		}
		return &remote{client: client, fullPath: full, settings: settings, maxBranches: f.MaxBranches}, nil
	}
	return nil, knowledgetypes.InvalidSource
}
