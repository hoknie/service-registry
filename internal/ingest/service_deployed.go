package ingest

import (
	"maps"
	"slices"
	"strings"
)

const (
	MetadataMaxPairs = 32
	MetadataValueMax = 1024
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
	}
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
