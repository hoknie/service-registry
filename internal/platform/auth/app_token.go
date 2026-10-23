package auth

import "svc-registry/pkg/apikey"

const (
	AppAccessPrefix  = "svco_"
	AppRefreshPrefix = "svcf_"
)

func GenerateAppAccessToken() (IssuedKey, error) { return apikey.Generate(AppAccessPrefix) }

func GenerateAppRefreshToken() (IssuedKey, error) { return apikey.Generate(AppRefreshPrefix) }

func ParseAppAccessToken(presented string) ([32]byte, bool) {
	return apikey.Parse(AppAccessPrefix, presented)
}

func ParseAppRefreshToken(presented string) ([32]byte, bool) {
	return apikey.Parse(AppRefreshPrefix, presented)
}
