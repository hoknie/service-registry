package deploy

import "context"

type Access struct {
	InCluster bool
	APIURL    string
	CAPEM     []byte
	Token     string
}

type ClusterClient interface {
	Version(ctx context.Context) (string, error)
	Missing(ctx context.Context, namespaces []string) ([]string, error)
	Workloads(ctx context.Context, namespaces []string) ([]Workload, error)
	Pods(ctx context.Context, namespace, selector string) ([]Pod, error)
	Jobs(ctx context.Context, namespace, cronJobUID string) ([]Job, error)
}

type ClientFactory interface {
	Client(a Access) (ClusterClient, error)
}
