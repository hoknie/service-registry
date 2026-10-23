package responses

import (
	deploytypes "svc-registry/internal/feature/deploy/types"
)

func AgentConfigOf(c deploytypes.AgentConfig) deploytypes.AgentConfig {
	if c.Namespaces == nil {
		c.Namespaces = []string{}
	}
	if c.Rules == nil {
		c.Rules = []deploytypes.Rule{}
	}
	return c
}

func ReportAnswerOf(a deploytypes.ReportAnswer) deploytypes.ReportAnswer {
	a.AgentConfig = AgentConfigOf(a.AgentConfig)
	return a
}
