package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	svcapp "svc-registry/internal/app"
	deployservices "svc-registry/internal/feature/deploy/services"
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/presentation/jobs"
	"svc-registry/internal/testsupport/k8sfake"
)

func (a *testApp) agentRunner(k8s *k8sfake.Factory, cluster, token string) *deployservices.AgentRunner {
	a.t.Helper()
	id, err := uuid.Parse(cluster)
	if err != nil {
		a.t.Fatal(err)
	}
	r, err := svcapp.OpenAgent(svcapp.AgentSettings{URL: "http://" + a.addr, Cluster: id, Token: token,
		Outbound: a.cfg.Outbound, K8s: a.cfg.K8s.K8sClientConfig}, svcapp.WithK8s(k8s))
	if err != nil {
		a.t.Fatal(err)
	}
	return r
}

func TestAgentSendsOneReport(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t)
	root := app.admin()
	app.projectWithKey(root, "api")
	c, token := app.agentCluster(root, "edge", obj{"rules": []obj{{"label": "app.kubernetes.io/name", "project": "api-org/{label:app.kubernetes.io/name}"}}})
	byRule := workload("u1", "api", "1.4.2", nil)
	other := workload("u2", "redis", "7", nil)
	other.Labels = map[string]string{"tier": "cache"}
	k8s.Cluster().Put(byRule, other)
	k8s.Cluster().PutPods("backend", "app=api", deploytypes.Pod{Containers: []deploytypes.PodContainer{{Name: "app", ImageID: "x@sha256:bb", Restarts: 1}}})
	k8s.Cluster().PutPods("backend", "app=redis", deploytypes.Pod{})
	k8s.Cluster().Missing("jobs.batch:list")

	runner := app.agentRunner(k8s, idOf(c), token)
	if runner.Health().Ready() {
		t.Fatal("ready before the first contact")
	}
	if err := runner.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	eq(t, runner.Health().Ready(), true)
	eq(t, runner.Health().Live(), true)
	got := app.get(clusterPath(idOf(c)), root).json(t)
	eq(t, got["status"], any("ok"))
	eq(t, at(got, "agent", "version"), any(svcapp.Version))
	eq(t, got["workloads"], any(float64(1)))
	eq(t, got["unmatched"], any(float64(1)))
	eq(t, scalar[string](t, app.db, "SELECT images->0->>'digest' FROM cluster_workloads WHERE uid = 'u1'"), "sha256:bb")
	test := app.call("POST", clusterPath(idOf(c))+"/test", root).json(t)
	eq(t, jsonText(test["missing"]), `["jobs.batch:list"]`)
	eq(t, test["version"], any("v1.31.2"))

	k8s.Cluster().Fail(&deploytypes.Failure{Code: deploytypes.FailForbidden, Message: "denied"})
	app.allowReport()
	if err := app.agentRunner(k8s, idOf(c), token).Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	got = app.get(clusterPath(idOf(c)), root).json(t)
	eq(t, at(got, "last_error", "code"), any("k8s.forbidden"))
	eq(t, got["workloads"], any(float64(1)))
}

func TestAgentStopsWhenTheTokenIsRefused(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t)
	root := app.admin()
	c, token := app.agentCluster(root, "edge", nil)
	eq(t, app.call("DELETE", clusterPath(idOf(c))+"/agent-token", root).status, 204)
	for _, once := range []bool{true, false} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := app.agentRunner(k8s, idOf(c), token).Run(ctx, once)
		cancel()
		if !errors.Is(err, deploytypes.ErrAgentRefused) {
			t.Fatalf("once=%v: %v", once, err)
		}
	}
	eq(t, deploytypes.ErrAgentRefused.Error(), "the registry refused the agent token")

	other, tok := app.agentCluster(root, "off", obj{"enabled": false})
	if err := app.agentRunner(k8s, idOf(other), tok).Run(context.Background(), true); !errors.Is(err, deploytypes.ErrClusterOffline) {
		t.Fatalf("disabled: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.agentRunner(k8s, idOf(other), tok).Run(ctx, false) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("a stopped agent exits cleanly: %v", err)
	}
}

func TestAgentClustersAreNotPolled(t *testing.T) {
	t.Parallel()
	app, k8s := startClusterApp(t)
	root := app.admin()
	app.agentCluster(root, "edge", nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := jobs.SpawnClusterPolls(ctx, app.services.Jobs())
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	eq(t, k8s.Cluster().Calls(), 0)
	eq(t, len(k8s.Accesses()), 0)
}

func TestAgentCommandLine(t *testing.T) {
	t.Parallel()
	code, _, errOut := run(t, []string{"agent", "--cluster", newID(), "--token-env", "SVCR_AGENT_TOKEN"}, "")
	eq(t, code, 2, errOut)
	contains(t, errOut, `required flag(s) "url" not set`)
	cluster := []string{"agent", "--url", "https://registry.example", "--cluster", newID(), "--token-env", "SVCR_AGENT_TOKEN"}
	code, _, errOut = run(t, cluster, "", "SVCR_AGENT_TOKEN", "svca_x")
	eq(t, code, 1, errOut)
	contains(t, errOut, "not running in a Kubernetes pod")
	code, _, errOut = run(t, cluster, "")
	eq(t, code, 1, errOut)
	contains(t, errOut, "SVCR_AGENT_TOKEN is not set")
	code, _, errOut = run(t, append(cluster, "--once", "--health-addr", ":0"), "", "SVCR_AGENT_TOKEN", "svca_x")
	eq(t, code, 2, errOut)
	contains(t, errOut, "health-addr")
	code, _, errOut = run(t, cluster, "", "SVCR_AGENT_TOKEN", "svca_x", "K8S_LIST_LIMIT", "10", "HTTP_ADDR", "nowhere")
	eq(t, code, 1, errOut)
	contains(t, errOut, "K8S_LIST_LIMIT")
	lacks(t, errOut, "DATABASE_URL")
	lacks(t, errOut, "HTTP_ADDR")
}
