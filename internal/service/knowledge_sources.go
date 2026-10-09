package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/docsource"
	"svc-registry/internal/forge"
	"svc-registry/internal/knowledge"
	"svc-registry/pkg/secretbox"
)

func GetKnowledgeSource(ctx context.Context, state *State, p Principal, id uuid.UUID) (*knowledge.Source, error) {
	if _, err := knowledgeProject(ctx, state, p, catalog.PermRead, id); err != nil {
		return nil, err
	}
	src, err := state.Sources.Get(ctx, id)
	if err != nil || src == nil {
		return nil, apperr.Wrap(err)
	}
	return &src.Source, nil
}

func validSource(state *State, in knowledge.SourceInput) (knowledge.ValidSource, error) {
	v, err := knowledge.ValidateSource(in)
	if err != nil {
		return knowledge.ValidSource{}, apperr.Wrap(err)
	}
	if v.IsLocal() {
		if err := docsource.Allowed(state.Config.Knowledge.LocalRoots, v.Path); err != nil {
			return knowledge.ValidSource{}, apperr.Wrap(err)
		}
	}
	return v, nil
}

func prepareSource(state *State, id uuid.UUID, in knowledge.SourceInput) (knowledge.NewSource, error) {
	v, err := validSource(state, in)
	if err != nil {
		return knowledge.NewSource{}, err
	}
	n := knowledge.NewSource{Source: v.Source, CredentialsRef: v.Reference, Keep: v.Keep}
	if v.Token != nil {
		if !state.Secrets.CanEncrypt() {
			return knowledge.NewSource{}, apperr.Wrap(forge.ConflictSecretsKeyMissing)
		}
		enc, err := state.Secrets.Seal(*v.Token, secretbox.AAD(knowledge.SourceTable, id.String(), knowledge.SourceColumnCredentials))
		if err != nil {
			return knowledge.NewSource{}, apperr.Internalf("encrypt: %v", err)
		}
		fp := secretbox.Fingerprint(*v.Token)
		n.CredentialsEnc, n.Fingerprint = &enc, &fp
	}
	return n, nil
}

func sourceRepo(s knowledge.Source) (f *catalog.Forge, url *string, ok bool) {
	if s.Kind != knowledge.SourceRemote {
		return nil, nil, false
	}
	f, ferr := catalog.ValidateForge(s.Forge)
	url, uerr := catalog.ValidateRepoURL(s.URL)
	return f, url, ferr == nil && uerr == nil && f != nil && url != nil
}

func PutKnowledgeSource(ctx context.Context, state *State, p Principal, id uuid.UUID, in knowledge.SourceInput) (knowledge.Source, error) {
	if _, err := knowledgeProject(ctx, state, p, catalog.PermWrite, id); err != nil {
		return knowledge.Source{}, err
	}
	n, err := prepareSource(state, id, in)
	if err != nil {
		return knowledge.Source{}, err
	}
	saved, err := state.Sources.Put(ctx, id, n)
	if err != nil {
		return knowledge.Source{}, apperr.Wrap(err)
	}
	f, url, ok := sourceRepo(saved)
	if !ok {
		return saved, nil
	}
	managed, err := state.Repositories.Managed(ctx, []uuid.UUID{id})
	if err != nil {
		return knowledge.Source{}, apperr.Wrap(err)
	}
	if !managed[id] {
		changes := catalog.NodeChanges{Forge: catalog.Change[catalog.Forge]{Set: true, Value: f}, RepoURL: catalog.Change[string]{Set: true, Value: url}}
		if _, err := state.Nodes.Update(ctx, id, changes); err != nil {
			return knowledge.Source{}, apperr.Wrap(err)
		}
	}
	return saved, nil
}

func DeleteKnowledgeSource(ctx context.Context, state *State, p Principal, id uuid.UUID) error {
	if _, err := knowledgeProject(ctx, state, p, catalog.PermWrite, id); err != nil {
		return err
	}
	return apperr.Wrap(state.Sources.Delete(ctx, id))
}

type SourceCheck struct {
	OK        bool
	Branches  int
	ErrorCode string
}

func CheckKnowledgeSource(ctx context.Context, state *State, p Principal, id uuid.UUID, in knowledge.SourceInput) (SourceCheck, error) {
	kp, err := knowledgeProject(ctx, state, p, catalog.PermWrite, id)
	if err != nil {
		return SourceCheck{}, err
	}
	v, err := validSource(state, in)
	if err != nil {
		return SourceCheck{}, err
	}
	token := ""
	switch {
	case v.Token != nil:
		token = *v.Token
	case v.Reference != nil:
		t, err := sourceToken(state, id, &knowledge.StoredSource{CredentialsRef: v.Reference})
		if err != nil {
			code, _ := failureOf(err)
			return SourceCheck{ErrorCode: code}, nil
		}
		token = t
	case v.Keep:
		if stored, err := state.Sources.Get(ctx, id); err == nil && stored != nil && stored.Kind == v.Kind && stored.URL == v.URL {
			t, err := sourceToken(state, id, stored)
			if err != nil {
				code, _ := failureOf(err)
				return SourceCheck{ErrorCode: code}, nil
			}
			token = t
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(state.Config.Outbound.TimeoutSecs)*time.Second)
	defer cancel()
	reader, err := state.DocReaders.Open(ctx, v.Source, kp.Settings, token)
	if err != nil {
		code, _ := failureOf(err)
		return SourceCheck{ErrorCode: code}, nil
	}
	heads, _, err := reader.Heads(ctx)
	if err != nil {
		code, _ := failureOf(err)
		return SourceCheck{ErrorCode: code}, nil
	}
	return SourceCheck{OK: true, Branches: len(heads)}, nil
}
