package deploy

import (
	"strings"

	"svc-registry/internal/catalog"
)

const (
	AnnotationProject     = "svc-registry.io/project"
	AnnotationService     = "svc-registry.io/service"
	AnnotationEnvironment = "svc-registry.io/environment"
	AnnotationBranch      = "svc-registry.io/branch"
	AnnotationCommit      = "svc-registry.io/commit"
	LabelVersion          = "app.kubernetes.io/version"
)

type Resolver func(ref string) (*ProjectRef, error)

type Match struct {
	Project     *ProjectRef
	Reason      Reason
	Annotation  *string
	Service     string
	Environment string
	Branch      *string
	Commit      *string
	Version     *string
}

func MatchWorkload(w Workload, clusterEnv string, rules []Rule, resolve Resolver) (Match, error) {
	m := Match{Version: VersionOf(w)}
	ann := func(key string) (string, bool) {
		if v, ok := w.Annotations[key]; ok {
			return v, true
		}
		v, ok := w.TemplateAnnotations[key]
		return v, ok
	}
	if b, ok := ann(AnnotationBranch); ok {
		if name, err := catalog.BranchName(b); err == nil {
			m.Branch = &name
		}
	}
	if c, ok := ann(AnnotationCommit); ok {
		if c = strings.ToLower(strings.TrimSpace(c)); commitOK(c) {
			m.Commit = &c
		}
	}
	if ref, ok := ann(AnnotationProject); ok {
		ref = strings.TrimSpace(ref)
		m.Annotation = &ref
		p, err := resolve(strings.Trim(ref, "/"))
		if err != nil {
			return Match{}, err
		}
		if p == nil {
			m.Reason = ReasonProjectNotFound
			return m, nil
		}
		m.Project = p
	} else {
		for _, r := range rules {
			path, ok := ruleTarget(r, w)
			if !ok {
				continue
			}
			p, err := resolve(path)
			if err != nil {
				return Match{}, err
			}
			if p != nil {
				m.Project = p
				break
			}
		}
		if m.Project == nil {
			m.Reason = ReasonNoAnnotation
			return m, nil
		}
	}
	service := w.Name
	if s, ok := ann(AnnotationService); ok {
		service = s
	}
	var ok bool
	if m.Service, ok = nameOf(service); !ok {
		return unmatched(m, ReasonInvalidService), nil
	}
	env := clusterEnv
	if e, found := ann(AnnotationEnvironment); found {
		env = e
	}
	if m.Environment, ok = nameOf(env); !ok {
		return unmatched(m, ReasonInvalidEnvironment), nil
	}
	return m, nil
}

func unmatched(m Match, r Reason) Match {
	m.Project, m.Reason = nil, r
	return m
}

func nameOf(raw string) (string, bool) {
	k, err := EnvironmentKey(raw)
	return k, err == nil
}

func ruleTarget(r Rule, w Workload) (string, bool) {
	if _, ok := w.Labels[r.Label]; !ok {
		return "", false
	}
	missing := false
	path := placeholderRe.ReplaceAllStringFunc(r.Project, func(ph string) string {
		inner := ph[1 : len(ph)-1]
		switch {
		case inner == "namespace":
			return w.Namespace
		case inner == "name":
			return w.Name
		default:
			v, ok := w.Labels[strings.TrimPrefix(inner, "label:")]
			if !ok {
				missing = true
			}
			return strings.ToLower(v)
		}
	})
	return path, !missing
}

func VersionOf(w Workload) *string {
	for _, labels := range []map[string]string{w.Labels, w.TemplateLabels} {
		if v := strings.TrimSpace(labels[LabelVersion]); v != "" {
			return &v
		}
	}
	if len(w.Containers) == 0 {
		return nil
	}
	ref := w.Containers[0].Image
	name, digest, _ := strings.Cut(ref, "@")
	slash := strings.LastIndex(name, "/")
	if colon := strings.LastIndex(name, ":"); colon > slash {
		tag := name[colon+1:]
		return &tag
	}
	if digest != "" {
		return &digest
	}
	return nil
}

func commitOK(c string) bool {
	if len(c) < 7 || len(c) > 64 {
		return false
	}
	for i := 0; i < len(c); i++ {
		if (c[i] < '0' || c[i] > '9') && (c[i] < 'a' || c[i] > 'f') {
			return false
		}
	}
	return true
}
