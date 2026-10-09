package deploy

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"svc-registry/internal/ingest"
)

var Locales = []string{"en", "es", "ru", "zh"}

const MaxPosition = 10_000

type Environment struct {
	ID        uuid.UUID
	Key       string
	Names     map[string]string
	Position  int32
	CreatedAt string
	UpdatedAt string
}

func EnvironmentKey(raw string) (string, error) {
	k, ok := ingest.Name(raw)
	if !ok {
		return "", InvalidEnvironmentKey
	}
	return k, nil
}

func ValidateNames(raw map[string]string) (map[string]string, error) {
	if len(raw) != len(Locales) {
		return nil, InvalidEnvironmentNames
	}
	out := make(map[string]string, len(Locales))
	for _, l := range Locales {
		v, ok := raw[l]
		if !ok {
			return nil, InvalidEnvironmentNames
		}
		v = strings.TrimFunc(v, unicode.IsSpace)
		if n := utf8.RuneCountInString(v); n < 1 || n > 100 || strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return nil, InvalidEnvironmentNames
		}
		out[l] = v
	}
	return out, nil
}

func ValidatePosition(raw int64) (int32, error) {
	if raw < 0 || raw > MaxPosition {
		return 0, InvalidPosition
	}
	return int32(raw), nil
}

func ValidateNewEnvironment(in CreateEnvironment) (NewEnvironment, error) {
	key, err := EnvironmentKey(in.Key)
	if err != nil {
		return NewEnvironment{}, err
	}
	names, err := ValidateNames(in.Names)
	if err != nil {
		return NewEnvironment{}, err
	}
	out := NewEnvironment{ID: uuid.Must(uuid.NewV7()), Key: key, Names: names}
	if in.Position != nil {
		if out.Position, err = ValidatePosition(*in.Position); err != nil {
			return NewEnvironment{}, err
		}
	}
	return out, nil
}

func ValidateEnvironmentChanges(in UpdateEnvironment) (EnvironmentChanges, error) {
	var out EnvironmentChanges
	if in.Names != nil {
		names, err := ValidateNames(in.Names)
		if err != nil {
			return EnvironmentChanges{}, err
		}
		out.Names = names
	}
	if in.Position != nil {
		p, err := ValidatePosition(*in.Position)
		if err != nil {
			return EnvironmentChanges{}, err
		}
		out.Position = &p
	}
	return out, nil
}

func EnvironmentOrder(directory []Environment) func(a, b string) int {
	pos := make(map[string]int32, len(directory))
	for _, e := range directory {
		pos[e.Key] = e.Position
	}
	return func(a, b string) int {
		pa, okA := pos[a]
		pb, okB := pos[b]
		switch {
		case okA && okB:
			return cmp.Or(cmp.Compare(pa, pb), strings.Compare(a, b))
		case okA:
			return -1
		case okB:
			return 1
		}
		return strings.Compare(a, b)
	}
}

func SortEnvironments(keys []string, directory []Environment) {
	slices.SortFunc(keys, EnvironmentOrder(directory))
}
