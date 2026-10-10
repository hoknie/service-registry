package links

import (
	"cmp"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

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
	Title        *string
	IconURL      *string
	IconFile     *string
	KindIcon     string
	CreatedAt    string
	UpdatedAt    string
	Inherited    bool
}

type LinkIcon struct {
	Kind string
	Name string
	URL  string
}

func (t Template) LinkIcon() LinkIcon {
	switch {
	case t.IconURL != nil:
		return LinkIcon{Kind: "url", URL: *t.IconURL}
	case t.IconFile != nil:
		return LinkIcon{Kind: "file", URL: "/api/v1/link-icons/" + *t.IconFile}
	}
	return LinkIcon{Kind: "builtin", Name: t.KindIcon}
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
	if out.Title, err = validateTitle(in.Title); err != nil {
		return NewTemplate{}, err
	}
	if out.IconURL, out.IconFile, err = validateIcon(in.Icon); err != nil {
		return NewTemplate{}, err
	}
	return out, nil
}

var iconFile = regexp.MustCompile(`^[0-9a-f]{64}\.(png|webp|ico|svg)$`)

func validateTitle(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	title := strings.TrimSpace(*raw)
	if title == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(title) > 100 || strings.IndexFunc(title, unicode.IsControl) >= 0 {
		return nil, InvalidLinkTitle
	}
	return &title, nil
}

func validateIcon(in *IconInput) (*string, *string, error) {
	switch {
	case in == nil:
		return nil, nil, nil
	case in.Err != nil:
		return nil, nil, in.Err
	case in.URL != nil && in.File == nil:
		u, err := url.Parse(*in.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" || len(*in.URL) > 2048 {
			return nil, nil, InvalidTemplateIcon
		}
		return in.URL, nil, nil
	case in.File != nil && in.URL == nil && iconFile.MatchString(*in.File):
		return nil, in.File, nil
	}
	return nil, nil, InvalidTemplateIcon
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
