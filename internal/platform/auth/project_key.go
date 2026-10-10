package auth

import "svc-registry/pkg/apikey"

const (
	KeyPrefix = "svcr_"
	KeyLen    = len(KeyPrefix) + apikey.BodyLen + apikey.CheckLen
)

func GenerateProjectKey() (IssuedKey, error) { return apikey.Generate(KeyPrefix) }

func ParseProjectKey(presented string) ([32]byte, bool) { return apikey.Parse(KeyPrefix, presented) }
