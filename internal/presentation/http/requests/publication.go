package requests

import (
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	publishingtypes "svc-registry/internal/feature/publishing/types"
)

type PublicationVersion struct {
	Pattern *string `json:"pattern"`
	Kind    *string `json:"kind"`
}

type Publication struct {
	Publish        Optional[bool]                 `json:"publish"`
	Versions       Optional[[]PublicationVersion] `json:"versions"`
	DefaultBranch  Optional[bool]                 `json:"default_branch"`
	CurrentRelease Optional[bool]                 `json:"current_release"`
	Name           Optional[string]               `json:"name"`
	Host           Optional[string]               `json:"host"`
}

func (r Publication) Settings() (publishingtypes.NodeSettings, error) {
	var out publishingtypes.NodeSettings
	for _, f := range []struct {
		in  Optional[bool]
		out **bool
	}{{r.Publish, &out.Publish}, {r.DefaultBranch, &out.DefaultBranch}, {r.CurrentRelease, &out.CurrentRelease}} {
		if !f.in.Present {
			continue
		}
		if f.in.Value == nil {
			return publishingtypes.NodeSettings{}, publishingtypes.InvalidPublication
		}
		*f.out = f.in.Value
	}
	if r.Versions.Present {
		if r.Versions.Value == nil {
			return publishingtypes.NodeSettings{}, publishingtypes.InvalidPublication
		}
		versions := make([]publishingtypes.Version, len(*r.Versions.Value))
		for i, v := range *r.Versions.Value {
			if v.Pattern == nil || v.Kind == nil {
				return publishingtypes.NodeSettings{}, publishingtypes.InvalidPublication
			}
			versions[i] = publishingtypes.Version{Pattern: *v.Pattern, Kind: knowledgetypes.RefKind(*v.Kind)}
		}
		out.Versions = &versions
	}
	out.Name, out.Host = r.Name.Value, r.Host.Value
	return out, nil
}
