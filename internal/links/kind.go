package links

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"svc-registry/internal/catalog"
)

var Locales = []string{"en", "es", "ru", "zh"}

type Icon string

var Icons = []Icon{"link", "logs", "dashboard", "errors", "alerts", "traces", "runbook", "docs"}

const DefaultIcon Icon = "link"

const MaxPosition = 10_000

type Names map[string]string

type Kind struct {
	ID        uuid.UUID
	Key       string
	Names     Names
	Icon      Icon
	Position  int32
	CreatedAt string
	UpdatedAt string
}

func ValidateKey(raw string) (string, error) {
	k, err := catalog.ValidateSlug(raw)
	if err != nil {
		return "", InvalidKindKey
	}
	return k, nil
}

func ValidateNames(raw map[string]string) (Names, error) {
	if len(raw) != len(Locales) {
		return nil, InvalidKindNames
	}
	out := make(Names, len(Locales))
	for _, l := range Locales {
		v, ok := raw[l]
		if !ok {
			return nil, InvalidKindNames
		}
		v = strings.TrimFunc(v, unicode.IsSpace)
		if n := utf8.RuneCountInString(v); n < 1 || n > 100 || strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return nil, InvalidKindNames
		}
		out[l] = v
	}
	return out, nil
}

func ValidateIcon(raw string) (Icon, error) {
	for _, i := range Icons {
		if string(i) == raw {
			return i, nil
		}
	}
	return "", InvalidIcon
}

func ValidatePosition(raw int64) (int32, error) {
	if raw < 0 || raw > MaxPosition {
		return 0, InvalidPosition
	}
	return int32(raw), nil
}

func ValidateNewKind(in CreateKind) (NewKind, error) {
	key, err := ValidateKey(in.Key)
	if err != nil {
		return NewKind{}, err
	}
	names, err := ValidateNames(in.Names)
	if err != nil {
		return NewKind{}, err
	}
	out := NewKind{ID: uuid.Must(uuid.NewV7()), Key: key, Names: names, Icon: DefaultIcon}
	if in.Icon != nil {
		if out.Icon, err = ValidateIcon(*in.Icon); err != nil {
			return NewKind{}, err
		}
	}
	if in.Position != nil {
		if out.Position, err = ValidatePosition(*in.Position); err != nil {
			return NewKind{}, err
		}
	}
	return out, nil
}

func ValidateKindChanges(in UpdateKind) (KindChanges, error) {
	var out KindChanges
	if in.Names != nil {
		names, err := ValidateNames(in.Names)
		if err != nil {
			return KindChanges{}, err
		}
		out.Names = names
	}
	if in.Icon != nil {
		icon, err := ValidateIcon(*in.Icon)
		if err != nil {
			return KindChanges{}, err
		}
		out.Icon = &icon
	}
	if in.Position != nil {
		pos, err := ValidatePosition(*in.Position)
		if err != nil {
			return KindChanges{}, err
		}
		out.Position = &pos
	}
	return out, nil
}
