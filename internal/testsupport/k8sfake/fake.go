package k8sfake

import (
	"context"
	"sync"

	"svc-registry/internal/feature/deploy"
)

type Cluster struct {
	mu        sync.Mutex
	workloads []deploy.Workload
	pods      map[string][]deploy.Pod
	jobs      map[string][]deploy.Job
	version   string
	missing   []string
	fail      *deploy.Failure
	token     string
	calls     int
}

type Factory struct {
	mu       sync.Mutex
	cluster  *Cluster
	accesses []deploy.Access
}

func New(token string) *Factory {
	return &Factory{cluster: &Cluster{pods: map[string][]deploy.Pod{}, jobs: map[string][]deploy.Job{}, version: "v1.31.2",
		missing: []string{}, token: token}}
}

func (f *Factory) Cluster() *Cluster { return f.cluster }

func (f *Factory) Accesses() []deploy.Access {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]deploy.Access(nil), f.accesses...)
}

func (f *Factory) Client(a deploy.Access) (deploy.ClusterClient, error) {
	f.mu.Lock()
	f.accesses = append(f.accesses, a)
	f.mu.Unlock()
	if !a.InCluster && a.Token != f.cluster.token {
		return client{c: f.cluster, unauthorized: true}, nil
	}
	return client{c: f.cluster}, nil
}

func (c *Cluster) Put(ws ...deploy.Workload) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workloads = ws
}

func (c *Cluster) PutPods(namespace, selector string, pods ...deploy.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pods[namespace+"/"+selector] = pods
}

func (c *Cluster) PutJobs(cronJobUID string, jobs ...deploy.Job) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobs[cronJobUID] = jobs
}

func (c *Cluster) Missing(m ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.missing = m
}

func (c *Cluster) Fail(f *deploy.Failure) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fail = f
}

func (c *Cluster) Calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

type client struct {
	c            *Cluster
	unauthorized bool
}

func (cl client) err() error {
	if cl.unauthorized {
		return &deploy.Failure{Code: deploy.FailUnauthorized, Message: "Unauthorized"}
	}
	if cl.c.fail != nil {
		return cl.c.fail
	}
	return nil
}

func (cl client) Version(context.Context) (string, error) {
	cl.c.mu.Lock()
	defer cl.c.mu.Unlock()
	if err := cl.err(); err != nil {
		return "", err
	}
	return cl.c.version, nil
}

func (cl client) Missing(context.Context, []string) ([]string, error) {
	cl.c.mu.Lock()
	defer cl.c.mu.Unlock()
	if err := cl.err(); err != nil {
		return nil, err
	}
	return append([]string{}, cl.c.missing...), nil
}

func (cl client) Workloads(_ context.Context, namespaces []string) ([]deploy.Workload, error) {
	cl.c.mu.Lock()
	defer cl.c.mu.Unlock()
	cl.c.calls++
	if err := cl.err(); err != nil {
		return nil, err
	}
	var out []deploy.Workload
	for _, w := range cl.c.workloads {
		if len(namespaces) == 0 || contains(namespaces, w.Namespace) {
			out = append(out, w)
		}
	}
	return out, nil
}

func (cl client) Pods(_ context.Context, namespace, selector string) ([]deploy.Pod, error) {
	cl.c.mu.Lock()
	defer cl.c.mu.Unlock()
	if err := cl.err(); err != nil {
		return nil, err
	}
	return cl.c.pods[namespace+"/"+selector], nil
}

func (cl client) Jobs(_ context.Context, _, cronJobUID string) ([]deploy.Job, error) {
	cl.c.mu.Lock()
	defer cl.c.mu.Unlock()
	if err := cl.err(); err != nil {
		return nil, err
	}
	return cl.c.jobs[cronJobUID], nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
