package k8s

import (
	"context"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var needed = []struct{ group, resource string }{
	{"apps", "deployments"}, {"apps", "statefulsets"}, {"apps", "daemonsets"}, {"apps", "replicasets"},
	{"batch", "cronjobs"}, {"batch", "jobs"}, {"", "pods"},
}

func (c *Client) Missing(ctx context.Context, namespaces []string) ([]string, error) {
	missing := []string{}
	for _, n := range needed {
		allowed := false
		for _, ns := range scopes(namespaces) {
			review := &authv1.SelfSubjectAccessReview{Spec: authv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authv1.ResourceAttributes{Namespace: ns, Verb: "list", Group: n.group, Resource: n.resource},
			}}
			r, err := c.cs.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
			if err != nil {
				return nil, failure(err)
			}
			if r.Status.Allowed {
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
