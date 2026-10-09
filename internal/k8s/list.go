package k8s

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"svc-registry/internal/deploy"
)

func pages(ctx context.Context, limit int64, list func(ctx context.Context, opts metav1.ListOptions) (string, error)) error {
	opts := metav1.ListOptions{Limit: limit}
	for {
		next, err := list(ctx, opts)
		if err != nil {
			return failure(err)
		}
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
	apps, batch := c.cs.AppsV1(), c.cs.BatchV1()
	for _, ns := range scopes(namespaces) {
		err := pages(ctx, c.limit, func(ctx context.Context, o metav1.ListOptions) (string, error) {
			l, err := apps.Deployments(ns).List(ctx, o)
			if err != nil {
				return "", err
			}
			for i := range l.Items {
				out = append(out, fromDeployment(&l.Items[i]))
			}
			return l.Continue, nil
		})
		if err == nil {
			err = pages(ctx, c.limit, func(ctx context.Context, o metav1.ListOptions) (string, error) {
				l, err := apps.StatefulSets(ns).List(ctx, o)
				if err != nil {
					return "", err
				}
				for i := range l.Items {
					out = append(out, fromStatefulSet(&l.Items[i]))
				}
				return l.Continue, nil
			})
		}
		if err == nil {
			err = pages(ctx, c.limit, func(ctx context.Context, o metav1.ListOptions) (string, error) {
				l, err := apps.DaemonSets(ns).List(ctx, o)
				if err != nil {
					return "", err
				}
				for i := range l.Items {
					out = append(out, fromDaemonSet(&l.Items[i]))
				}
				return l.Continue, nil
			})
		}
		if err == nil {
			err = pages(ctx, c.limit, func(ctx context.Context, o metav1.ListOptions) (string, error) {
				l, err := batch.CronJobs(ns).List(ctx, o)
				if err != nil {
					return "", err
				}
				for i := range l.Items {
					out = append(out, fromCronJob(&l.Items[i]))
				}
				return l.Continue, nil
			})
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (c *Client) Pods(ctx context.Context, namespace, selector string) ([]deploy.Pod, error) {
	var out []deploy.Pod
	err := pages(ctx, c.limit, func(ctx context.Context, o metav1.ListOptions) (string, error) {
		o.LabelSelector = selector
		l, err := c.cs.CoreV1().Pods(namespace).List(ctx, o)
		if err != nil {
			return "", err
		}
		for _, p := range l.Items {
			var pod deploy.Pod
			for _, s := range p.Status.ContainerStatuses {
				pod.Containers = append(pod.Containers, deploy.PodContainer{Name: s.Name, ImageID: s.ImageID, Restarts: s.RestartCount})
			}
			out = append(out, pod)
		}
		return l.Continue, nil
	})
	return out, err
}

func (c *Client) Jobs(ctx context.Context, namespace, cronJobUID string) ([]deploy.Job, error) {
	var out []deploy.Job
	err := pages(ctx, c.limit, func(ctx context.Context, o metav1.ListOptions) (string, error) {
		l, err := c.cs.BatchV1().Jobs(namespace).List(ctx, o)
		if err != nil {
			return "", err
		}
		for _, j := range l.Items {
			if !ownedBy(j.OwnerReferences, cronJobUID) {
				continue
			}
			job := deploy.Job{Active: j.Status.Active, Succeeded: j.Status.Succeeded, Failed: j.Status.Failed}
			if j.Status.StartTime != nil {
				t := j.Status.StartTime.Time
				job.StartedAt = &t
			}
			out = append(out, job)
		}
		return l.Continue, nil
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

func base(kind deploy.Kind, m metav1.ObjectMeta, tpl corev1.PodTemplateSpec, sel *metav1.LabelSelector) deploy.Workload {
	w := deploy.Workload{UID: string(m.UID), Kind: kind, Namespace: m.Namespace, Name: m.Name,
		Labels: m.Labels, Annotations: m.Annotations, TemplateLabels: tpl.Labels, TemplateAnnotations: tpl.Annotations,
		Generation: m.Generation}
	if sel != nil {
		w.Selector = metav1.FormatLabelSelector(sel)
	}
	for _, c := range tpl.Spec.Containers {
		w.Containers = append(w.Containers, deploy.Container{Name: c.Name, Image: c.Image})
	}
	return w
}

func replicas(p *int32) int32 {
	if p == nil {
		return 1
	}
	return *p
}

func fromDeployment(d *appsv1.Deployment) deploy.Workload {
	w := base(deploy.KindDeployment, d.ObjectMeta, d.Spec.Template, d.Spec.Selector)
	w.ObservedGeneration = d.Status.ObservedGeneration
	w.Desired, w.Ready, w.Updated = replicas(d.Spec.Replicas), d.Status.ReadyReplicas, d.Status.UpdatedReplicas
	for _, c := range d.Status.Conditions {
		w.Conditions = append(w.Conditions, deploy.Condition{Type: string(c.Type), Status: string(c.Status), Reason: c.Reason})
	}
	return w
}

func fromStatefulSet(s *appsv1.StatefulSet) deploy.Workload {
	w := base(deploy.KindStatefulSet, s.ObjectMeta, s.Spec.Template, s.Spec.Selector)
	w.ObservedGeneration = s.Status.ObservedGeneration
	w.Desired, w.Ready, w.Updated = replicas(s.Spec.Replicas), s.Status.ReadyReplicas, s.Status.UpdatedReplicas
	return w
}

func fromDaemonSet(d *appsv1.DaemonSet) deploy.Workload {
	w := base(deploy.KindDaemonSet, d.ObjectMeta, d.Spec.Template, d.Spec.Selector)
	w.ObservedGeneration = d.Status.ObservedGeneration
	w.Desired, w.Ready, w.Updated = d.Status.DesiredNumberScheduled, d.Status.NumberReady, d.Status.UpdatedNumberScheduled
	return w
}

func fromCronJob(c *batchv1.CronJob) deploy.Workload {
	w := base(deploy.KindCronJob, c.ObjectMeta, c.Spec.JobTemplate.Spec.Template, nil)
	w.Schedule = c.Spec.Schedule
	w.Suspended = c.Spec.Suspend != nil && *c.Spec.Suspend
	return w
}
