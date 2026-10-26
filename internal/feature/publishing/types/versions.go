package types

import knowledgetypes "svc-registry/internal/feature/knowledge/types"

type PublicVersion struct {
	Name        string
	Kind        knowledgetypes.RefKind
	CollectedAt string
	Release     bool
}

type PublicDocs struct {
	Default  *PublicVersion
	Branches []PublicVersion
	Tags     []PublicVersion
	Release  *string
}

func Pick(docs PublicDocs, sel knowledgetypes.RefSelector) *PublicVersion {
	switch {
	case sel.IsZero():
		return docs.Default
	case sel.Release:
		for i := range docs.Tags {
			if docs.Tags[i].Release {
				return &docs.Tags[i]
			}
		}
		return nil
	}
	list := docs.Branches
	if sel.Kind == knowledgetypes.RefTag {
		list = docs.Tags
	}
	for i := range list {
		if list[i].Name == sel.Name {
			return &list[i]
		}
	}
	return nil
}
