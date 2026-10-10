package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var accessReviews = schema.GroupVersionResource{Group: "authorization.k8s.io", Version: "v1", Resource: "selfsubjectaccessreviews"}

var needed = []struct{ group, resource string }{
	{"apps", "deployments"}, {"apps", "statefulsets"}, {"apps", "daemonsets"}, {"apps", "replicasets"},
	{"batch", "cronjobs"}, {"batch", "jobs"}, {"", "pods"},
}

func (c *Client) allowed(ctx context.Context, namespace, group, resource string) (bool, error) {
	review := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "authorization.k8s.io/v1", "kind": "SelfSubjectAccessReview",
		"spec": map[string]any{"resourceAttributes": map[string]any{"namespace": namespace, "verb": "list", "group": group, "resource": resource}},
	}}
	r, err := c.dyn.Resource(accessReviews).Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, failure(err)
	}
	return flag(r.Object, "status", "allowed"), nil
}

func (c *Client) Missing(ctx context.Context, namespaces []string) ([]string, error) {
	missing := []string{}
	for _, n := range needed {
		allowed := false
		for _, ns := range scopes(namespaces) {
			ok, err := c.allowed(ctx, ns, n.group, n.resource)
			if err != nil {
				return nil, err
			}
			if ok {
				allowed = true
				break
			}
		}
		if !allowed {
			name := n.resource
			if n.group != "" {
				name += "." + n.group
			}
			missing = append(missing, name+":list")
		}
	}
	return missing, nil
}
