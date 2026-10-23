package requests

import (
	deploytypes "svc-registry/internal/feature/deploy/types"
)

type AgentReport struct {
	deploytypes.Report
}

func (r AgentReport) Domain() deploytypes.Report { return r.Report }
