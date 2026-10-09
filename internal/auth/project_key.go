package auth

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/catalog"
	"svc-registry/pkg/apikey"
)

const (
	KeyPrefix = "svcr_"
	KeyLen    = len(KeyPrefix) + apikey.BodyLen + apikey.CheckLen
)

func GenerateProjectKey() (IssuedKey, error) { return apikey.Generate(KeyPrefix) }

func ParseProjectKey(presented string) ([32]byte, bool) { return apikey.Parse(KeyPrefix, presented) }

func VerifyProjectKey(ctx context.Context, keys catalog.ProjectKeyStore, projectID uuid.UUID, presented string) (*uuid.UUID, error) {
	hash, ok := ParseProjectKey(presented)
	if !ok {
		return nil, nil
	}
	return keys.Verify(ctx, projectID, hash)
}
