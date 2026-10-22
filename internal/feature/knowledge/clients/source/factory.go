package source

import (
	"context"

	forgeports "svc-registry/internal/feature/forge/ports"
	forgetypes "svc-registry/internal/feature/forge/types"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

type FactoryClient struct {
	Roots        []string
	Forges       forgeports.ClientFactoryPort
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
		client, err := f.Forges.New(forgetypes.Endpoint{Kind: forgetypes.Kind(s.Forge), APIURL: s.APIURL, Owner: owner(full), Token: token})
		if err != nil {
			return nil, err
		}
		return &remote{client: client, fullPath: full, settings: settings, maxBranches: f.MaxBranches}, nil
	}
	return nil, knowledgetypes.InvalidSource
}
