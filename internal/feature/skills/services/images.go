package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

type ImagesService struct{ content *ContentService }

func (s *ImagesService) view(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (skillstypes.Skill, error) {
	return s.content.view(ctx, p, id)
}

func (s *ImagesService) Image(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (string, []byte, error) {
	sk, err := s.view(ctx, p, id)
	if err != nil {
		return "", nil, err
	}
	if sk.HasUploadedImage && sk.ImageType != nil {
		data, err := s.content.read(ctx, skillstypes.File{SHA256: sk.ImageSHA256, Size: skillstypes.MaxImageBytes})
		return *sk.ImageType, data, err
	}
	if sk.ImagePath == nil || sk.Version == nil {
		return "", nil, apperr.New(apperr.NotFound)
	}
	files, err := s.content.versions.Files(ctx, sk.Version.ID)
	if err != nil {
		return "", nil, apperr.Wrap(err)
	}
	f, err := findFile(files, *sk.ImagePath)
	if err != nil || f.Kind != skillstypes.KindImage {
		return "", nil, apperr.New(apperr.NotFound)
	}
	data, err := s.content.read(ctx, f)
	return f.ContentType, data, err
}

func (s *ImagesService) PutImage(ctx context.Context, p accesstypes.Principal, id uuid.UUID, data []byte) (skillstypes.Skill, error) {
	if _, err := writeSkill(ctx, s.content.parts, p, id, accesstypes.ActionUpdate); err != nil {
		return skillstypes.Skill{}, err
	}
	if len(data) > skillstypes.MaxImageBytes {
		return skillstypes.Skill{}, apperr.Wrap(skillstypes.ImageTooLarge)
	}
	ct, ok := skillstypes.ImageType(data)
	if !ok {
		return skillstypes.Skill{}, apperr.Wrap(skillstypes.InvalidImage)
	}
	if s.content.store == nil {
		return skillstypes.Skill{}, apperr.Wrap(skillstypes.ConflictStorageOff)
	}
	sum := sha256.Sum256(data)
	if err := s.content.store.Put(ctx, hex.EncodeToString(sum[:]), int64(len(data)), bytes.NewReader(data)); err != nil {
		return skillstypes.Skill{}, apperr.Wrap(err)
	}
	err := s.content.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.content.blobs.Touch(ctx, [][]byte{sum[:]}, []int64{int64(len(data))}); err != nil {
			return err
		}
		return s.content.skills.SetImage(ctx, id, sum[:], &ct)
	})
	if err != nil {
		return skillstypes.Skill{}, apperr.Wrap(err)
	}
	return s.view(ctx, p, id)
}

func (s *ImagesService) DeleteImage(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if _, err := writeSkill(ctx, s.content.parts, p, id, accesstypes.ActionUpdate); err != nil {
		return err
	}
	return apperr.Wrap(s.content.skills.SetImage(ctx, id, nil, nil))
}
