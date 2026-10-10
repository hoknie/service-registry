package k8s

import (
	"context"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"svc-registry/internal/feature/deploy"
)

var (
	deployments  = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	statefulSets = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	daemonSets   = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	cronJobs     = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
	jobs         = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	pods         = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
)

func (c *Client) list(ctx context.Context, gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions,
	each func(u *unstructured.Unstructured)) error {
	opts.Limit = c.limit
	for {
		l, err := c.dyn.Resource(gvr).Namespace(namespace).List(ctx, opts)
		if err != nil {
			return failure(err)
		}
		for i := range l.Items {
			each(&l.Items[i])
		}
		next := l.GetContinue()
		if next == "" {
			return nil
		}
		opts.Continue = next
	}
}

func scopes(namespaces []string) []string {
	if len(namespaces) == 0 {
		return []string{metav1.NamespaceAll}
	}
	return namespaces
}

func (c *Client) Workloads(ctx context.Context, namespaces []string) ([]deploy.Workload, error) {
	var out []deploy.Workload
	kinds := []struct {
		gvr     schema.GroupVersionResource
		convert func(*unstructured.Unstructured) deploy.Workload
	}{{deployments, fromDeployment}, {statefulSets, fromStatefulSet}, {daemonSets, fromDaemonSet}, {cronJobs, fromCronJob}}
	for _, ns := range scopes(namespaces) {
		for _, k := range kinds {
			err := c.list(ctx, k.gvr, ns, metav1.ListOptions{}, func(u *unstructured.Unstructured) { out = append(out, k.convert(u)) })
			if err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func (c *Client) Pods(ctx context.Context, namespace, selector string) ([]deploy.Pod, error) {
	var out []deploy.Pod
	err := c.list(ctx, pods, namespace, metav1.ListOptions{LabelSelector: selector}, func(u *unstructured.Unstructured) {
		var pod deploy.Pod
		for _, s := range items(u.Object, "status", "containerStatuses") {
			pod.Containers = append(pod.Containers, deploy.PodContainer{Name: str(s, "name"), ImageID: str(s, "imageID"), Restarts: i32(s, "restartCount")})
		}
		out = append(out, pod)
	})
	return out, err
}

func (c *Client) Jobs(ctx context.Context, namespace, cronJobUID string) ([]deploy.Job, error) {
	var out []deploy.Job
	err := c.list(ctx, jobs, namespace, metav1.ListOptions{}, func(u *unstructured.Unstructured) {
		if !ownedBy(u.GetOwnerReferences(), cronJobUID) {
			return
		}
		out = append(out, deploy.Job{Active: i32(u.Object, "status", "active"), Succeeded: i32(u.Object, "status", "succeeded"),
			Failed: i32(u.Object, "status", "failed"), StartedAt: timestamp(u.Object, "status", "startTime")})
	})
	return out, err
}

func ownedBy(refs []metav1.OwnerReference, uid string) bool {
	for _, r := range refs {
		if string(r.UID) == uid {
			return true
		}
	}
	return false
}

func base(kind deploy.Kind, u *unstructured.Unstructured, template ...string) deploy.Workload {
	w := deploy.Workload{UID: string(u.GetUID()), Kind: kind, Namespace: u.GetNamespace(), Name: u.GetName(),
		Labels: u.GetLabels(), Annotations: u.GetAnnotations(), Generation: u.GetGeneration(),
		TemplateLabels:      stringMap(u.Object, slices.Concat(template, []string{"metadata", "labels"})...),
		TemplateAnnotations: stringMap(u.Object, slices.Concat(template, []string{"metadata", "annotations"})...)}
	for _, c := range items(u.Object, slices.Concat(template, []string{"spec", "containers"})...) {
		w.Containers = append(w.Containers, deploy.Container{Name: str(c, "name"), Image: str(c, "image")})
	}
	return w
}

func replicas(obj map[string]any) int32 {
	v, found, _ := unstructured.NestedInt64(obj, "spec", "replicas")
	if !found {
		return 1
	}
	return int32(v)
}

var podTemplate = []string{"spec", "template"}

func fromDeployment(u *unstructured.Unstructured) deploy.Workload {
	w := base(deploy.KindDeployment, u, podTemplate...)
	w.Selector = selector(u.Object, "spec", "selector")
	w.ObservedGeneration = i64(u.Object, "status", "observedGeneration")
	w.Desired, w.Ready, w.Updated = replicas(u.Object), i32(u.Object, "status", "readyReplicas"), i32(u.Object, "status", "updatedReplicas")
	for _, c := range items(u.Object, "status", "conditions") {
		w.Conditions = append(w.Conditions, deploy.Condition{Type: str(c, "type"), Status: str(c, "status"), Reason: str(c, "reason")})
	}
	return w
}

func fromStatefulSet(u *unstructured.Unstructured) deploy.Workload {
	w := base(deploy.KindStatefulSet, u, podTemplate...)
	w.Selector = selector(u.Object, "spec", "selector")
	w.ObservedGeneration = i64(u.Object, "status", "observedGeneration")
	w.Desired, w.Ready, w.Updated = replicas(u.Object), i32(u.Object, "status", "readyReplicas"), i32(u.Object, "status", "updatedReplicas")
	return w
}

func fromDaemonSet(u *unstructured.Unstructured) deploy.Workload {
	w := base(deploy.KindDaemonSet, u, podTemplate...)
	w.Selector = selector(u.Object, "spec", "selector")
	w.ObservedGeneration = i64(u.Object, "status", "observedGeneration")
	w.Desired = i32(u.Object, "status", "desiredNumberScheduled")
	w.Ready, w.Updated = i32(u.Object, "status", "numberReady"), i32(u.Object, "status", "updatedNumberScheduled")
	return w
}

func fromCronJob(u *unstructured.Unstructured) deploy.Workload {
	w := base(deploy.KindCronJob, u, "spec", "jobTemplate", "spec", "template")
	w.Schedule = str(u.Object, "spec", "schedule")
	w.Suspended = flag(u.Object, "spec", "suspend")
	return w
}
