package k8s

import (
	"context"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	authv1 "k8s.io/api/authorization/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"svc-registry/internal/config"
	"svc-registry/internal/deploy"
)

func ptr[T any](v T) *T { return &v }

func TestWorkloadsPodsAndJobsAreConverted(t *testing.T) {
	ctx := context.Background()
	cs := fake.NewClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "backend", UID: "d1", Generation: 2,
				Annotations: map[string]string{deploy.AnnotationProject: "acme/backend/api"}},
			Spec: appsv1.DeploymentSpec{Replicas: ptr(int32(3)), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
				Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "api"}},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "acme/api:1.4.2"}}}}},
			Status: appsv1.DeploymentStatus{ObservedGeneration: 2, ReadyReplicas: 2, UpdatedReplicas: 3,
				Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}},
		},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "backend", UID: "s1"}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "ops", UID: "ds1"},
			Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 5, NumberReady: 4, UpdatedNumberScheduled: 5}},
		&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "report", Namespace: "backend", UID: "c1"},
			Spec: batchv1.CronJobSpec{Schedule: "0 * * * *", Suspend: ptr(true)}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "report-1", Namespace: "backend",
			OwnerReferences: []metav1.OwnerReference{{UID: types.UID("c1")}}},
			Status: batchv1.JobStatus{Failed: 1, StartTime: &metav1.Time{Time: time.Unix(100, 0)}}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "backend"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "backend", Labels: map[string]string{"app": "api"}},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "app", ImageID: "acme/api@sha256:d", RestartCount: 4}}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "backend", Labels: map[string]string{"app": "x"}}},
	)
	c := NewClient(cs, 500)
	all, err := c.Workloads(ctx, nil)
	if err != nil || len(all) != 4 {
		t.Fatalf("%v %+v", err, all)
	}
	only, err := c.Workloads(ctx, []string{"ops"})
	if err != nil || len(only) != 1 || only[0].Kind != deploy.KindDaemonSet || only[0].Desired != 5 || only[0].Ready != 4 {
		t.Fatalf("%v %+v", err, only)
	}
	var d, cron deploy.Workload
	for _, w := range all {
		switch w.Kind {
		case deploy.KindDeployment:
			d = w
		case deploy.KindCronJob:
			cron = w
		}
	}
	if d.UID != "d1" || d.Desired != 3 || d.Ready != 2 || d.Selector != "app=api" || d.Containers[0].Image != "acme/api:1.4.2" ||
		d.Annotations[deploy.AnnotationProject] != "acme/backend/api" || d.Conditions[0].Reason != "ProgressDeadlineExceeded" {
		t.Fatalf("%+v", d)
	}
	if cron.Schedule != "0 * * * *" || !cron.Suspended {
		t.Fatalf("%+v", cron)
	}
	pods, err := c.Pods(ctx, "backend", d.Selector)
	if err != nil || len(pods) != 1 || pods[0].Containers[0].Restarts != 4 {
		t.Fatalf("%v %+v", err, pods)
	}
	jobs, err := c.Jobs(ctx, "backend", "c1")
	if err != nil || len(jobs) != 1 || jobs[0].Failed != 1 || jobs[0].StartedAt == nil {
		t.Fatalf("%v %+v", err, jobs)
	}
}

func TestMissingPermissions(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		r := a.(k8stesting.CreateAction).GetObject().(*authv1.SelfSubjectAccessReview)
		attrs := r.Spec.ResourceAttributes
		r.Status.Allowed = attrs.Resource != "pods" && !(attrs.Resource == "cronjobs" && attrs.Namespace == "a")
		return true, r, nil
	})
	missing, err := NewClient(cs, 500).Missing(context.Background(), []string{"a", "b"})
	if err != nil || len(missing) != 1 || missing[0] != "pods:list" {
		t.Fatalf("%v %v", err, missing)
	}
}

func apiServer(t *testing.T, unauthorized bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if unauthorized || r.Header.Get("Authorization") != "Bearer sa-token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"Unauthorized","code":401}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"major":"1","minor":"31","gitVersion":"v1.31.2"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func caOf(srv *httptest.Server) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
}

func failureCode(err error) deploy.FailureCode {
	var f *deploy.Failure
	if errors.As(err, &f) {
		return f.Code
	}
	return ""
}

func TestRealTransport(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	f := NewFactory(config.OutboundConfig{}, config.K8sConfig{RequestTimeoutSecs: 5, ListLimit: 500})
	srv := apiServer(t, false)
	c, err := f.Client(deploy.Access{APIURL: srv.URL, CAPEM: caOf(srv), Token: "sa-token"})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := c.Version(context.Background()); err != nil || v != "v1.31.2" {
		t.Fatalf("%q %v", v, err)
	}
	c, _ = f.Client(deploy.Access{APIURL: srv.URL, CAPEM: caOf(srv), Token: "wrong"})
	if _, err := c.Version(context.Background()); failureCode(err) != deploy.FailUnauthorized {
		t.Fatalf("%v", err)
	}
	c, _ = f.Client(deploy.Access{APIURL: srv.URL, Token: "sa-token"})
	if _, err := c.Version(context.Background()); failureCode(err) != deploy.FailTLS {
		t.Fatalf("%v", err)
	}
	c, _ = f.Client(deploy.Access{APIURL: "https://127.0.0.1:1", Token: "sa-token"})
	if _, err := c.Version(context.Background()); failureCode(err) != deploy.FailUnreachable {
		t.Fatalf("%v", err)
	}
}
