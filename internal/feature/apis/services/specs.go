package services

import (
	"context"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"

	accesstypes "svc-registry/internal/feature/access/types"
	apistypes "svc-registry/internal/feature/apis/types"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
)

type SpecsService struct {
	documents *knowledgeservices.DocumentsService
	cache     *lru.Cache[cacheKey, apistypes.Document]
}

type snapshotSpecs struct {
	files    knowledgetypes.SnapshotFiles
	set      *apistypes.Files
	skipped  map[string]knowledgetypes.SkipReason
	contents map[string]string
}

func (s *SpecsService) open(ctx context.Context, p accesstypes.Principal, id uuid.UUID, r knowledgeservices.KnowledgeRef) (*snapshotSpecs, error) {
	files, err := s.documents.SnapshotContents(ctx, p, id, r)
	if err != nil {
		return nil, err
	}
	out := &snapshotSpecs{files: files, skipped: map[string]knowledgetypes.SkipReason{}}
	var collected, candidates []string
	for _, f := range files.Files {
		if f.Skip != nil {
			out.skipped[f.Path] = *f.Skip
			continue
		}
		collected = append(collected, f.Path)
		if apistypes.IsCandidate(f.Path) && !apistypes.IsProto(f.Path) {
			candidates = append(candidates, f.Path)
		}
	}
	if out.contents, err = files.Contents(ctx, candidates); err != nil {
		return nil, err
	}
	out.set = apistypes.NewFiles(collected, func(paths []string) (map[string]string, error) {
		return files.Contents(ctx, paths)
	}, out.contents)
	return out, nil
}

func (sp *snapshotSpecs) isAPI(path string) bool {
	if _, skipped := sp.skipped[path]; skipped {
		_, ok := apistypes.DetectSkipped(path)
		return ok
	}
	_, _, ok := apistypes.Detect(path, sp.contents[path])
	return ok
}

func (s *SpecsService) document(ctx context.Context, sp *snapshotSpecs, path string) (apistypes.Document, bool, error) {
	if reason, ok := sp.skipped[path]; ok {
		format, isAPI := apistypes.DetectSkipped(path)
		if !isAPI {
			return apistypes.Document{}, false, nil
		}
		return apistypes.SkippedDocument(path, format, string(reason)), true, nil
	}
	if !sp.set.Has(path) || !apistypes.IsCandidate(path) {
		return apistypes.Document{}, false, nil
	}
	key := cacheKey{snapshot: sp.files.Snapshot.ID, path: path}
	if d, ok := s.cache.Get(key); ok {
		return d, true, nil
	}
	content, found, err := sp.set.Read(path)
	if err != nil || !found {
		return apistypes.Document{}, false, err
	}
	format, version, isAPI := apistypes.Detect(path, content)
	if !isAPI {
		return apistypes.Document{}, false, nil
	}
	var d apistypes.Document
	if format == apistypes.FormatProto {
		d, err = apistypes.ParseProto(ctx, path, content, sp.set)
	} else {
		d, err = apistypes.ParseOpenAPI(path, content, format, version, sp.set)
	}
	if err != nil {
		return apistypes.Document{}, false, apperr.Wrap(err)
	}
	s.cache.Add(key, d)
	return d, true, nil
}

type APIList struct {
	Snapshot       knowledgetypes.Snapshot
	Items          []apistypes.Summary
	HiddenVendored int
}

func (s *SpecsService) ListAPIs(ctx context.Context, p accesstypes.Principal, id uuid.UUID, r knowledgeservices.KnowledgeRef, all bool) (APIList, error) {
	sp, err := s.open(ctx, p, id, r)
	if err != nil {
		return APIList{}, err
	}
	out := APIList{Snapshot: sp.files.Snapshot, Items: []apistypes.Summary{}}
	for _, f := range sp.files.Files {
		if !apistypes.IsCandidate(f.Path) {
			continue
		}
		if !all && apistypes.Vendored(f.Path) {
			if sp.isAPI(f.Path) {
				out.HiddenVendored++
			}
			continue
		}
		d, ok, err := s.document(ctx, sp, f.Path)
		if err != nil {
			return APIList{}, err
		}
		if ok {
			out.Items = append(out.Items, d.Summary())
		}
	}
	return out, nil
}

func (s *SpecsService) GetAPI(ctx context.Context, p accesstypes.Principal, id uuid.UUID, r knowledgeservices.KnowledgeRef, path string) (knowledgetypes.Snapshot, apistypes.Document, error) {
	sp, err := s.open(ctx, p, id, r)
	if err != nil {
		return knowledgetypes.Snapshot{}, apistypes.Document{}, err
	}
	d, ok, err := s.document(ctx, sp, path)
	if err != nil {
		return knowledgetypes.Snapshot{}, apistypes.Document{}, err
	}
	if !ok {
		return knowledgetypes.Snapshot{}, apistypes.Document{}, apperr.New(apperr.NotFound)
	}
	return sp.files.Snapshot, d, nil
}
