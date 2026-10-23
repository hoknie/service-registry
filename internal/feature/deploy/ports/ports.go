package ports

import (
	"context"

	deploytypes "svc-registry/internal/feature/deploy/types"
)

type ClusterClientPort interface {
	Version(ctx context.Context) (string, error)
	Missing(ctx context.Context, namespaces []string) ([]string, error)
	Workloads(ctx context.Context, namespaces []string) ([]deploytypes.Workload, error)
	Pods(ctx context.Context, namespace, selector string) ([]deploytypes.Pod, error)
	Jobs(ctx context.Context, namespace, cronJobUID string) ([]deploytypes.Job, error)
}

type ClientFactoryPort interface {
	Client(a deploytypes.Access) (ClusterClientPort, error)
}

type RegistryClientPort interface {
	Config(ctx context.Context) (deploytypes.AgentConfig, error)
	Report(ctx context.Context, r deploytypes.Report) (deploytypes.ReportAnswer, error)
}
