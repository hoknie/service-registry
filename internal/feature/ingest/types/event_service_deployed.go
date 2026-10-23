package types

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	topologytypes "svc-registry/internal/feature/topology/types"
)

const (
	MetadataMaxPairs = 32
	MetadataValueMax = 1024
	LinksMax         = 20
)

type ServiceDeployedV1 struct {
	Service     Field[string]         `json:"service"`
	Version     Field[string]         `json:"version"`
	Environment Field[string]         `json:"environment"`
	CommitSHA   Field[string]         `json:"commit_sha"`
	Branch      Field[string]         `json:"branch"`
	Cluster     Field[string]         `json:"cluster"`
	Namespace   Field[string]         `json:"namespace"`
	URL         Field[string]         `json:"url"`
	DeployedBy  Field[string]         `json:"deployed_by"`
	Metadata    Field[map[string]any] `json:"metadata"`
	Links       Field[[]any]          `json:"links"`
	DependsOn   Field[[]any]          `json:"dependencies"`
}

var (
	nameRule       = rule{norm: lowerTrim, tags: "dotted"}
	versionRule    = rule{tags: "max=128,token"}
	commitRule     = rule{norm: strings.ToLower, tags: "min=7,max=64,hexadecimal"}
	branchRule     = rule{tags: "max=255,token,startsnotwith=-"}
	clusterRule    = rule{norm: trim, tags: "max=100,nocontrol"}
	namespaceRule  = rule{norm: trim, tags: "dnslabel"}
	urlRule        = rule{tags: "max=2048,token,http_url"}
	deployedByRule = rule{norm: trim, tags: "max=200,nocontrol"}
	metaValueTags  = "max=1024,nocontrol"
	linkKeyRule    = rule{norm: lowerTrim, tags: "dnslabel"}
	linkTitleRule  = rule{norm: trim, tags: "max=100,nocontrol"}
)

func (p ServiceDeployedV1) parse(errs *fields) ServiceDeployed {
	return ServiceDeployed{
		Service:     errs.required("payload.service", p.Service, nameRule),
		Version:     errs.required("payload.version", p.Version, versionRule),
		Environment: errs.required("payload.environment", p.Environment, nameRule),
		CommitSHA:   errs.optional("payload.commit_sha", p.CommitSHA, commitRule),
		Branch:      errs.optional("payload.branch", p.Branch, branchRule),
		Cluster:     errs.optional("payload.cluster", p.Cluster, clusterRule),
		Namespace:   errs.optional("payload.namespace", p.Namespace, namespaceRule),
		URL:         errs.optional("payload.url", p.URL, urlRule),
		DeployedBy:  errs.optional("payload.deployed_by", p.DeployedBy, deployedByRule),
		Metadata:    p.metadata(errs),
		Links:       p.links(errs),
		DependsOn:   p.dependencies(errs),
	}
}

func (p ServiceDeployedV1) dependencies(errs *fields) *[]topologytypes.Ref {
	switch p.DependsOn.State {
	case Absent, Null:
		return nil
	case WrongType:
		errs.add("payload.dependencies", FieldInvalidType)
		return nil
	}
	if len(p.DependsOn.Value) > topologytypes.MaxRefs {
		errs.add("payload.dependencies", FieldInvalidValue)
		return nil
	}
	var texts []string
	for i, raw := range p.DependsOn.Value {
		path := "payload.dependencies." + strconv.Itoa(i)
		s, ok := raw.(string)
		if !ok {
			errs.add(path, FieldInvalidType)
			continue
		}
		if _, err := topologytypes.ParseRef(s); err != nil {
			errs.add(path, FieldInvalidValue)
			continue
		}
		texts = append(texts, s)
	}
	refs, _ := topologytypes.ParseRefs(texts)
	if refs == nil {
		refs = []topologytypes.Ref{}
	}
	return &refs
}

func (p ServiceDeployedV1) links(errs *fields) []DeploymentLink {
	out := []DeploymentLink{}
	switch p.Links.State {
	case Absent, Null:
		return out
	case WrongType:
		errs.add("payload.links", FieldInvalidType)
		return out
	}
	if len(p.Links.Value) > LinksMax {
		errs.add("payload.links", FieldInvalidValue)
		return out
	}
	seen := map[string]bool{}
	for i, raw := range p.Links.Value {
		path := "payload.links." + strconv.Itoa(i)
		m, ok := raw.(map[string]any)
		if !ok {
			errs.add(path, FieldInvalidType)
			continue
		}
		before := len(*errs)
		kind := errs.required(path+".kind", stringField(m, "kind"), linkKeyRule)
		url := errs.required(path+".url", stringField(m, "url"), urlRule)
		key := errs.optional(path+".key", stringField(m, "key"), linkKeyRule)
		title := errs.optional(path+".title", stringField(m, "title"), linkTitleRule)
		if key == nil && kind != "" {
			key = &kind
		}
		if key != nil {
			if seen[*key] {
				errs.add(path+".key", FieldInvalidValue)
			}
			seen[*key] = true
		}
		if len(*errs) == before {
			out = append(out, DeploymentLink{Key: *key, Kind: kind, URL: url, Title: title})
		}
	}
	return out
}

func stringField(m map[string]any, name string) Field[string] {
	v, ok := m[name]
	switch {
	case !ok:
		return Field[string]{State: Absent}
	case v == nil:
		return Field[string]{State: Null}
	}
	s, ok := v.(string)
	if !ok {
		return Field[string]{State: WrongType}
	}
	return Field[string]{Value: s, State: Set}
}

func (p ServiceDeployedV1) metadata(errs *fields) map[string]string {
	out := map[string]string{}
	switch p.Metadata.State {
	case Absent, Null:
		return out
	case WrongType:
		errs.add("payload.metadata", FieldInvalidType)
		return out
	}
	m := p.Metadata.Value
	if len(m) > MetadataMaxPairs {
		errs.add("payload.metadata", FieldInvalidValue)
		return out
	}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		path := "payload.metadata." + k
		s, isString := m[k].(string)
		switch {
		case !valid(k, "dotted"):
			errs.add(path, FieldInvalidValue)
		case !isString:
			errs.add(path, FieldInvalidType)
		case !valid(s, metaValueTags):
			errs.add(path, FieldInvalidValue)
		default:
			out[k] = s
		}
	}
	return out
}
