package usecases

import (
	"bytes"
	"context"
	"encoding/hex"

	"github.com/google/uuid"

	skillsports "svc-registry/internal/feature/skills/ports"
	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

type SaveVersionDeps struct {
	DB       *pgtx.DB
	Skills   *repositories.SkillRepository
	Versions *repositories.VersionRepository
	Blobs    *repositories.BlobRepository
	Store    skillsports.BlobStorePort
	Keep     int
}

type SaveVersionUseCase struct{ d SaveVersionDeps }

func NewSaveVersion(d SaveVersionDeps) *SaveVersionUseCase { return &SaveVersionUseCase{d: d} }

type SaveInput struct {
	SkillID     uuid.UUID
	New         *repositories.NewSkill
	Content     skillstypes.Content
	FormVersion string
	Origin      skillstypes.Origin
	Commit      *string
	Tag         *string
	CreatedBy   *uuid.UUID
	Author      string
	ManagedTags bool
	Policy      func(skillID uuid.UUID) skillstypes.Effective
}

type SaveResult struct {
	Version skillstypes.Version
	Created bool
}

func (u *SaveVersionUseCase) Run(ctx context.Context, in SaveInput) (SaveResult, error) {
	if u.d.Store == nil {
		return SaveResult{}, skillstypes.ConflictStorageOff
	}
	if err := u.store(ctx, in.Content); err != nil {
		return SaveResult{}, err
	}
	var out SaveResult
	err := u.d.DB.InTx(ctx, func(ctx context.Context) error {
		var err error
		out, err = u.save(ctx, in)
		return err
	})
	return out, repositories.Err(err)
}

func (u *SaveVersionUseCase) store(ctx context.Context, c skillstypes.Content) error {
	for _, f := range c.Files {
		if err := u.d.Store.Put(ctx, hex.EncodeToString(f.SHA256), f.Size, bytes.NewReader(f.Data)); err != nil {
			return err
		}
	}
	return nil
}

func (u *SaveVersionUseCase) save(ctx context.Context, in SaveInput) (SaveResult, error) {
	m := in.Content.Manifest
	if in.New != nil {
		in.New.Manifest = m
		if in.New.Manifest.Author == "" {
			in.New.Manifest.Author = in.Author
		}
		if err := u.d.Skills.Insert(ctx, *in.New); err != nil {
			return SaveResult{}, err
		}
		in.SkillID = in.New.ID
	} else if ok, err := u.d.Skills.LockOne(ctx, in.SkillID); err != nil || !ok {
		return SaveResult{}, notFound(err)
	}
	current, err := u.d.Skills.Get(ctx, in.SkillID)
	if err != nil || current == nil {
		return SaveResult{}, notFound(err)
	}
	if current.Name != m.Name {
		return SaveResult{}, skillstypes.ConflictName
	}
	byID, err := u.d.Versions.Of(ctx, []uuid.UUID{in.SkillID})
	if err != nil {
		return SaveResult{}, err
	}
	existing := byID[in.SkillID]
	version, explicit, err := skillstypes.ResolveVersion(in.FormVersion, m.Version, labels(existing))
	if err != nil {
		return SaveResult{}, err
	}
	if same, done, err := reuse(existing, version, explicit, in.Content.Digest); done {
		return SaveResult{Version: same}, err
	}
	saved, err := u.insert(ctx, in, version)
	if err != nil {
		return SaveResult{}, err
	}
	all := append(existing, saved)
	if err := u.card(ctx, in, *current, all); err != nil {
		return SaveResult{}, err
	}
	return SaveResult{Version: saved, Created: true}, u.prune(ctx, in, all)
}

func notFound(err error) error {
	if err != nil {
		return err
	}
	return apperr.New(apperr.NotFound)
}

func labels(versions []skillstypes.Version) []string {
	out := make([]string, len(versions))
	for i, v := range versions {
		out[i] = v.Version
	}
	return out
}

func reuse(existing []skillstypes.Version, version string, explicit bool, digest string) (skillstypes.Version, bool, error) {
	for _, v := range existing {
		if v.Version == version {
			if v.Digest == digest {
				return v, true, nil
			}
			return skillstypes.Version{}, true, skillstypes.ConflictVersion
		}
	}
	if !explicit && len(existing) > 0 {
		sorted := append([]skillstypes.Version(nil), existing...)
		skillstypes.SortDesc(sorted)
		if sorted[0].Digest == digest {
			return sorted[0], true, nil
		}
	}
	return skillstypes.Version{}, false, nil
}

func (u *SaveVersionUseCase) insert(ctx context.Context, in SaveInput, version string) (skillstypes.Version, error) {
	files := make([]skillstypes.File, len(in.Content.Files))
	sums, sizes := make([][]byte, len(files)), make([]int64, len(files))
	for i, f := range in.Content.Files {
		files[i], sums[i], sizes[i] = f.File, f.SHA256, f.Size
	}
	if err := u.d.Blobs.Touch(ctx, sums, sizes); err != nil {
		return skillstypes.Version{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return skillstypes.Version{}, err
	}
	nv := repositories.NewVersion{ID: id, SkillID: in.SkillID, Version: version, Parts: skillstypes.PartsOf(version),
		Origin: in.Origin, Commit: in.Commit, Tag: in.Tag, Digest: in.Content.Digest, Files: files, Size: in.Content.Size,
		Manifest: in.Content.Manifest, CreatedBy: in.CreatedBy}
	if err := u.d.Versions.Insert(ctx, nv); err != nil {
		return skillstypes.Version{}, err
	}
	return skillstypes.Version{ID: id, SkillID: in.SkillID, Version: version, Origin: in.Origin, Commit: in.Commit, Tag: in.Tag,
		Files: len(files), Size: in.Content.Size, Digest: in.Content.Digest, Manifest: in.Content.Manifest}, nil
}

func (u *SaveVersionUseCase) card(ctx context.Context, in SaveInput, current skillstypes.Skill, all []skillstypes.Version) error {
	sorted := append([]skillstypes.Version(nil), all...)
	skillstypes.SortDesc(sorted)
	top := sorted[0].Manifest
	author := top.Author
	if author == "" {
		author = in.Author
		if author == "" {
			author = current.Author
		}
	}
	var tags []string
	if in.ManagedTags {
		tags = top.Tags
		if tags == nil {
			tags = []string{}
		}
	}
	return u.d.Skills.SetCard(ctx, in.SkillID, top, author, tags)
}

func (u *SaveVersionUseCase) prune(ctx context.Context, in SaveInput, all []skillstypes.Version) error {
	if u.d.Keep <= 0 || len(all) <= u.d.Keep {
		return nil
	}
	sorted := append([]skillstypes.Version(nil), all...)
	if in.Policy != nil {
		eff := in.Policy(in.SkillID)
		sorted = eff.Mark(sorted, true)
	} else {
		skillstypes.SortDesc(sorted)
	}
	var drop []uuid.UUID
	for i, v := range sorted {
		if i >= u.d.Keep && !v.Available {
			drop = append(drop, v.ID)
		}
	}
	if len(drop) == 0 {
		return nil
	}
	return u.d.Versions.Delete(ctx, drop)
}
