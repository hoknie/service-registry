package types

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
)

var Locales = []string{"en", "es", "ru", "zh"}

const DefaultIcon = "link"

const (
	MaxPosition     = 10_000
	MaxIconNameLen  = 64
	MaxIconURLLen   = 2048
	MaxKindNameLen  = 100
	MaxReorderedKey = 1000
)

var iconName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Names map[string]string

type Kind struct {
	ID        uuid.UUID
	Key       string
	Names     Names
	Icon      string
	IconURL   *string
	Position  int32
	CreatedAt string
	UpdatedAt string
}

type KindInfo struct {
	Position int32
	Icon     string
	IconURL  *string
}

func (k Kind) Info() KindInfo {
	return KindInfo{Position: k.Position, Icon: k.Icon, IconURL: k.IconURL}
}

func ValidateKey(raw string) (string, error) {
	k, err := catalogtypes.ValidateSlug(raw)
	if err != nil {
		return "", InvalidKindKey
	}
	return k, nil
}

func ValidateNames(raw map[string]*string) (Names, error) {
	given := make(map[string]string, len(Locales))
	for l, v := range raw {
		if !isLocale(l) {
			return nil, InvalidKindNames
		}
		if v == nil {
			continue
		}
		s := strings.TrimFunc(*v, unicode.IsSpace)
		if s == "" {
			continue
		}
		if utf8.RuneCountInString(s) > MaxKindNameLen || strings.IndexFunc(s, unicode.IsControl) >= 0 {
			return nil, InvalidKindNames
		}
		given[l] = s
	}
	fallback := ""
	for _, l := range Locales {
		if v, ok := given[l]; ok {
			fallback = v
			break
		}
	}
	if fallback == "" {
		return nil, InvalidKindNames
	}
	out := make(Names, len(Locales))
	for _, l := range Locales {
		if v, ok := given[l]; ok {
			out[l] = v
		} else {
			out[l] = fallback
		}
	}
	return out, nil
}

func isLocale(l string) bool {
	for _, x := range Locales {
		if x == l {
			return true
		}
	}
	return false
}

func ValidateIcon(raw string) (string, error) {
	if len(raw) > MaxIconNameLen || !iconName.MatchString(raw) {
		return "", InvalidIcon
	}
	return raw, nil
}

func ValidateIconURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || len(raw) > MaxIconURLLen ||
		strings.IndexFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return "", InvalidIcon
	}
	return raw, nil
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
	if in.IconURL != nil {
		u, err := ValidateIconURL(*in.IconURL)
		if err != nil {
			return NewKind{}, err
		}
		out.IconURL = &u
	}
	if in.Position != nil {
		pos, err := ValidatePosition(*in.Position)
		if err != nil {
			return NewKind{}, err
		}
		out.Position = &pos
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
	if in.IconURLSet {
		out.IconURLSet = true
		if in.IconURL != nil {
			u, err := ValidateIconURL(*in.IconURL)
			if err != nil {
				return KindChanges{}, err
			}
			out.IconURL = &u
		}
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

func ValidateOrder(keys []string) ([]string, error) {
	if keys == nil || len(keys) > MaxReorderedKey {
		return nil, InvalidKindOrder
	}
	seen := make(map[string]bool, len(keys))
	out := make([]string, 0, len(keys))
	for _, raw := range keys {
		k, err := ValidateKey(raw)
		if err != nil || seen[k] {
			return nil, InvalidKindOrder
		}
		seen[k] = true
		out = append(out, k)
	}
	return out, nil
}

func CheckOrderCurrent(current, keys []string) error {
	if len(current) != len(keys) {
		return ConflictKindOrderStale
	}
	given := make(map[string]bool, len(keys))
	for _, k := range keys {
		given[k] = true
	}
	for _, k := range current {
		if !given[k] {
			return ConflictKindOrderStale
		}
	}
	return nil
}
