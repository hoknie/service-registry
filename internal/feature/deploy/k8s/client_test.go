package k8s

import (
	"context"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/platform/config"
)

func ptr[T any](v T) *T { return &v }

var listKinds = map[schema.GroupVersionResource]string{
	deployments: "DeploymentList", statefulSets: "StatefulSetList", daemonSets: "DaemonSetList",
	cronJobs: "CronJobList", jobs: "JobList", pods: "PodList",
}

func object(t *testing.T, apiVersion, kind string, obj runtime.Object) *unstructured.Unstructured {
	t.Helper()
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	u := &unstructured.Unstructured{Object: m}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	return u
}

func fakeCluster(objects ...runtime.Object) *dynfake.FakeDynamicClient {
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objects...)
}

func fixtures(t *testing.T) []runtime.Object {
	return []runtime.Object{
		object(t, "apps/v1", "Deployment", &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "backend", UID: "d1", Generation: 2,
				Labels: map[string]string{"team": "core"}, Annotations: map[string]string{deploy.AnnotationProject: "acme/backend/api"}},
			Spec: appsv1.DeploymentSpec{Replicas: ptr(int32(3)), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"},
				MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "tier", Operator: metav1.LabelSelectorOpIn, Values: []string{"web"}}}},
				Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "api"},
					Annotations: map[string]string{deploy.AnnotationService: "api"}},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "acme/api:1.4.2"}, {Name: "proxy", Image: "envoy:1"}}}}},
			Status: appsv1.DeploymentStatus{ObservedGeneration: 2, ReadyReplicas: 2, UpdatedReplicas: 3,
				Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded"}}},
		}),
		object(t, "apps/v1", "StatefulSet", &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "backend", UID: "s1"},
			Status: appsv1.StatefulSetStatus{ObservedGeneration: 1, ReadyReplicas: 1, UpdatedReplicas: 1}}),
		object(t, "apps/v1", "DaemonSet", &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "ops", UID: "ds1"},
			Spec:   appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "agent"}}},
			Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 5, NumberReady: 4, UpdatedNumberScheduled: 5}}),
		object(t, "batch/v1", "CronJob", &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "report", Namespace: "backend", UID: "c1"},
			Spec: batchv1.CronJobSpec{Schedule: "0 * * * *", Suspend: ptr(true), JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"job": "report"}},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "run", Image: "acme/report:7"}}}}}}}}),
		object(t, "batch/v1", "Job", &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "report-1", Namespace: "backend",
			OwnerReferences: []metav1.OwnerReference{{UID: types.UID("c1"), Name: "report", Kind: "CronJob", APIVersion: "batch/v1"}}},
			Status: batchv1.JobStatus{Active: 1, Succeeded: 2, Failed: 1, StartTime: &metav1.Time{Time: time.Unix(100, 0)}}}),
		object(t, "batch/v1", "Job", &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "backend"}}),
		object(t, "v1", "Pod", &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "backend", Labels: map[string]string{"app": "api"}},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "app", ImageID: "acme/api@sha256:d", RestartCount: 4}}}}),
		object(t, "v1", "Pod", &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "backend", Labels: map[string]string{"app": "x"}}}),
	}
}

func TestWorkloadsPodsAndJobsAreConverted(t *testing.T) {
	ctx := context.Background()
	c := NewClient(fakeCluster(fixtures(t)...), http.DefaultClient, "", 500)
	all, err := c.Workloads(ctx, nil)
	if err != nil || len(all) != 4 {
		t.Fatalf("%v %+v", err, all)
	}
	byKind := map[deploy.Kind]deploy.Workload{}
	for _, w := range all {
		byKind[w.Kind] = w
	}
	want := map[deploy.Kind]deploy.Workload{
		deploy.KindDeployment: {UID: "d1", Kind: deploy.KindDeployment, Namespace: "backend", Name: "api",
			Labels: map[string]string{"team": "core"}, Annotations: map[string]string{deploy.AnnotationProject: "acme/backend/api"},
			TemplateLabels: map[string]string{"app": "api"}, TemplateAnnotations: map[string]string{deploy.AnnotationService: "api"},
			Selector:   "app=api,tier in (web)",
			Containers: []deploy.Container{{Name: "app", Image: "acme/api:1.4.2"}, {Name: "proxy", Image: "envoy:1"}},
			Generation: 2, ObservedGeneration: 2, Desired: 3, Ready: 2, Updated: 3,
			Conditions: []deploy.Condition{{Type: "Progressing", Status: "False", Reason: "ProgressDeadlineExceeded"}}},
		deploy.KindStatefulSet: {UID: "s1", Kind: deploy.KindStatefulSet, Namespace: "backend", Name: "db",
			ObservedGeneration: 1, Desired: 1, Ready: 1, Updated: 1},
		deploy.KindDaemonSet: {UID: "ds1", Kind: deploy.KindDaemonSet, Namespace: "ops", Name: "agent", Selector: "app=agent",
			Desired: 5, Ready: 4, Updated: 5},
		deploy.KindCronJob: {UID: "c1", Kind: deploy.KindCronJob, Namespace: "backend", Name: "report",
			TemplateLabels: map[string]string{"job": "report"}, Containers: []deploy.Container{{Name: "run", Image: "acme/report:7"}},
			Schedule: "0 * * * *", Suspended: true},
	}
	for kind, w := range want {
		if got := byKind[kind]; !reflect.DeepEqual(got, w) {
			t.Errorf("%s:\n got  %+v\n want %+v", kind, got, w)
		}
	}
	only, err := c.Workloads(ctx, []string{"ops"})
	if err != nil || len(only) != 1 || only[0].Kind != deploy.KindDaemonSet {
		t.Fatalf("%v %+v", err, only)
	}
	pods, err := c.Pods(ctx, "backend", "app=api")
	if err != nil || !reflect.DeepEqual(pods, []deploy.Pod{{Containers: []deploy.PodContainer{{Name: "app", ImageID: "acme/api@sha256:d", Restarts: 4}}}}) {
		t.Fatalf("%v %+v", err, pods)
	}
	jobs, err := c.Jobs(ctx, "backend", "c1")
	started := time.Unix(100, 0).UTC()
	if err != nil || len(jobs) != 1 || jobs[0].Active != 1 || jobs[0].Succeeded != 2 || jobs[0].Failed != 1 ||
		jobs[0].StartedAt == nil || !jobs[0].StartedAt.Equal(started) {
		t.Fatalf("%v %+v", err, jobs)
	}
}

func TestListsFollowContinueTokens(t *testing.T) {
	cs := fakeCluster()
	calls := 0
	cs.PrependReactor("list", "deployments", func(a k8stesting.Action) (bool, runtime.Object, error) {
		calls++
		opts := a.(k8stesting.ListActionImpl).ListOptions
		if opts.Limit != 2 {
			t.Fatalf("limit %d", opts.Limit)
		}
		l := &unstructured.UnstructuredList{Object: map[string]any{"apiVersion": "apps/v1", "kind": "DeploymentList"}}
		name := "first"
		if opts.Continue == "" {
			l.SetContinue("page-2")
		} else if opts.Continue == "page-2" {
			name = "second"
		}
		u := unstructured.Unstructured{Object: map[string]any{"apiVersion": "apps/v1", "kind": "Deployment"}}
		u.SetName(name)
		l.Items = append(l.Items, u)
		return true, l, nil
	})
	all, err := NewClient(cs, http.DefaultClient, "", 2).Workloads(context.Background(), []string{"ns"})
	if err != nil || calls != 2 || len(all) != 2 || all[0].Name != "first" || all[1].Name != "second" || all[1].Desired != 1 {
		t.Fatalf("%v %d %+v", err, calls, all)
	}
}

func TestListErrorsBecomeFailures(t *testing.T) {
	cases := map[string]struct {
		err  error
		want deploy.FailureCode
	}{
		"forbidden":    {apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "deployments"}, "", errors.New("no")), deploy.FailForbidden},
		"unauthorized": {apierrors.NewUnauthorized("who"), deploy.FailUnauthorized},
		"timeout":      {apierrors.NewTimeoutError("slow", 1), deploy.FailTimeout},
	}
	for name, tc := range cases {
		cs := fakeCluster()
		cs.PrependReactor("list", "*", func(k8stesting.Action) (bool, runtime.Object, error) { return true, nil, tc.err })
		_, err := NewClient(cs, http.DefaultClient, "", 500).Workloads(context.Background(), nil)
		if failureCode(err) != tc.want {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestMissingPermissions(t *testing.T) {
	cs := fakeCluster()
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		r := a.(k8stesting.CreateAction).GetObject().(*unstructured.Unstructured)
		resource, _, _ := unstructured.NestedString(r.Object, "spec", "resourceAttributes", "resource")
		namespace, _, _ := unstructured.NestedString(r.Object, "spec", "resourceAttributes", "namespace")
		verb, _, _ := unstructured.NestedString(r.Object, "spec", "resourceAttributes", "verb")
		if verb != "list" {
			t.Fatalf("verb %q", verb)
		}
		allowed := resource != "pods" && !(resource == "cronjobs" && namespace == "a")
		_ = unstructured.SetNestedField(r.Object, allowed, "status", "allowed")
		return true, r, nil
	})
	missing, err := NewClient(cs, http.DefaultClient, "", 500).Missing(context.Background(), []string{"a", "b"})
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
