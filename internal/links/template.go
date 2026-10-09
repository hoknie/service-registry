package links

import (
	"cmp"
	"slices"

	"github.com/google/uuid"
)

const MaxTemplatesPerNode = 50

type Template struct {
	ID           uuid.UUID
	NodeID       uuid.UUID
	LinkKey      string
	KindKey      string
	KindPosition int32
	Template     *string
	Disabled     bool
	Position     int32
	CreatedAt    string
	UpdatedAt    string
	Inherited    bool
}

func ValidateTemplate(nodeID uuid.UUID, rawLinkKey string, in PutTemplate) (NewTemplate, error) {
	linkKey, err := ValidateKey(rawLinkKey)
	if err != nil {
		return NewTemplate{}, err
	}
	kindKey, err := ValidateKey(in.KindKey)
	if err != nil {
		return NewTemplate{}, UnknownKind
	}
	out := NewTemplate{ID: uuid.Must(uuid.NewV7()), NodeID: nodeID, LinkKey: linkKey, KindKey: kindKey}
	out.Disabled = in.Disabled != nil && *in.Disabled
	switch {
	case out.Disabled && in.Template != nil && *in.Template != "":
		return NewTemplate{}, InvalidTemplate
	case !out.Disabled && in.Template == nil:
		return NewTemplate{}, InvalidTemplate
	case !out.Disabled:
		if err := CheckTemplate(*in.Template); err != nil {
			return NewTemplate{}, err
		}
		out.Template = in.Template
	}
	if in.Position != nil {
		if out.Position, err = ValidatePosition(*in.Position); err != nil {
			return NewTemplate{}, err
		}
	}
	return out, nil
}

func Effective(byNode [][]Template) []Template {
	seen := map[string]bool{}
	var out []Template
	for depth, own := range byNode {
		for _, t := range own {
			if seen[t.LinkKey] {
				continue
			}
			seen[t.LinkKey] = true
			t.Inherited = depth > 0
			out = append(out, t)
		}
	}
	SortTemplates(out)
	return out
}

func SortTemplates(ts []Template) {
	slices.SortFunc(ts, func(a, b Template) int {
		return cmp.Or(cmp.Compare(a.KindPosition, b.KindPosition), cmp.Compare(a.Position, b.Position),
			cmp.Compare(a.LinkKey, b.LinkKey))
	})
}
