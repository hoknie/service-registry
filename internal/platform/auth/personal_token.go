package auth

import "svc-registry/pkg/apikey"

const TokenPrefix = "svcp_"

func GeneratePersonalToken() (IssuedKey, error) { return apikey.Generate(TokenPrefix) }

func ParsePersonalToken(presented string) ([32]byte, bool) {
	return apikey.Parse(TokenPrefix, presented)
}
