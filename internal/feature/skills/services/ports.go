package services

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"

	skillsports "svc-registry/internal/feature/skills/ports"
	"svc-registry/internal/platform/apperr"
)

const blobGraceSecs = 24 * 60 * 60

type PortsService struct{ parts }

func (s *PortsService) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	out, err := s.sources.UsingSecrets(ctx, secrets)
	return out, apperr.Wrap(err)
}

func (s *PortsService) PruneBlobs(ctx context.Context) (int, error) {
	if s.store == nil {
		return 0, nil
	}
	sums, err := s.blobs.Unreferenced(ctx, blobGraceSecs, 500)
	if err != nil {
		return 0, apperr.Wrap(err)
	}
	removed := 0
	for _, sum := range sums {
		forgotten, err := s.blobs.Forget(ctx, sum, blobGraceSecs)
		if err != nil {
			return removed, apperr.Wrap(err)
		}
		if !forgotten {
			continue
		}
		if err := s.store.Delete(ctx, hex.EncodeToString(sum)); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

type CopyResult struct {
	Copied  int
	Skipped int
}

func (s *PortsService) CopyBlobs(ctx context.Context, from, to skillsports.BlobStorePort) (CopyResult, error) {
	var out CopyResult
	var after []byte
	for {
		sums, err := s.blobs.All(ctx, after, 500)
		if err != nil {
			return out, apperr.Wrap(err)
		}
		if len(sums) == 0 {
			return out, nil
		}
		for _, sum := range sums {
			if err := copyOne(ctx, s.parts, from, to, sum, &out); err != nil {
				return out, err
			}
		}
		after = sums[len(sums)-1]
	}
}

var ErrBlobMissing = errors.New("a skill file is missing in SKILLS_DIR")

func copyOne(ctx context.Context, p parts, from, to skillsports.BlobStorePort, sum []byte, out *CopyResult) error {
	key := hex.EncodeToString(sum)
	has, err := to.Has(ctx, key)
	if err != nil {
		return err
	}
	if has {
		out.Skipped++
		return nil
	}
	size, err := p.blobs.Size(ctx, sum)
	if err != nil || size == nil {
		return apperr.Wrap(err)
	}
	rc, err := from.Open(ctx, key)
	if errors.Is(err, apperr.ErrNotFound) {
		return errors.Join(ErrBlobMissing, errors.New(key))
	}
	if err != nil {
		return err
	}
	defer rc.Close()
	if err := to.Put(ctx, key, *size, rc); err != nil {
		return err
	}
	out.Copied++
	return nil
}
