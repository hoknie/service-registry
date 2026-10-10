package links

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
)

const (
	MaxVars     = 32
	MaxVarValue = 1024
)

type Var struct {
	Key       string
	Value     string
	NodeID    uuid.UUID
	Inherited bool
}

func ValidateVars(raw map[string]string) (map[string]string, error) {
	if len(raw) > MaxVars {
		return nil, InvalidVars
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if !catalog.IsLabelKey(k) || utf8.RuneCountInString(v) > MaxVarValue || strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return nil, InvalidVars
		}
		out[k] = v
	}
	return out, nil
}

func EffectiveVars(byNode [][]Var) []Var {
	seen := map[string]bool{}
	var out []Var
	for depth, own := range byNode {
		for _, v := range own {
			if seen[v.Key] {
				continue
			}
			seen[v.Key] = true
			v.Inherited = depth > 0
			out = append(out, v)
		}
	}
	slices.SortFunc(out, func(a, b Var) int { return strings.Compare(a.Key, b.Key) })
	return out
}
