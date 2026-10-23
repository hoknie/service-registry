package auth

import "svc-registry/pkg/apikey"

const AgentTokenPrefix = "svca_"

func GenerateAgentToken() (IssuedKey, error) { return apikey.Generate(AgentTokenPrefix) }

func ParseAgentToken(presented string) ([32]byte, bool) {
	return apikey.Parse(AgentTokenPrefix, presented)
}
