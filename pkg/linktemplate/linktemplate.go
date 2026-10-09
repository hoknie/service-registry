package linktemplate

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxLen = 2048

type Error struct {
	UnknownVariable bool
	Pos             int
	Detail          string
}

func (e *Error) Error() string { return fmt.Sprintf("template: %s at character %d", e.Detail, e.Pos) }

type filter int

const (
	filterURL filter = iota + 1
	filterRaw
	filterLower
)

type part struct {
	text    string
	name    string
	filters []filter
}

type Pattern struct{ parts []part }

func (p *Pattern) Names() []string {
	var out []string
	for _, pt := range p.parts {
		if pt.name != "" && !slices.Contains(out, pt.name) {
			out = append(out, pt.name)
		}
	}
	return out
}

func Parse(template string, known func(name string) bool) (*Pattern, error) {
	if n := utf8.RuneCountInString(template); n < 1 || n > MaxLen {
		return nil, &Error{Pos: 1, Detail: "length must be 1 to 2048 characters"}
	}
	if i := strings.IndexFunc(template, unicode.IsControl); i >= 0 {
		return nil, &Error{Pos: pos(template, i), Detail: "control character"}
	}
	var parts []part
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			parts = append(parts, part{text: text.String()})
			text.Reset()
		}
	}
	for i := 0; i < len(template); {
		switch c := template[i]; {
		case c == '{' && strings.HasPrefix(template[i:], "{{"):
			text.WriteByte('{')
			i += 2
		case c == '}' && strings.HasPrefix(template[i:], "}}"):
			text.WriteByte('}')
			i += 2
		case c == '}':
			return nil, &Error{Pos: pos(template, i), Detail: `unmatched "}" (write "}}" for a literal brace)`}
		case c == '{':
			end := strings.IndexByte(template[i:], '}')
			if end < 0 {
				return nil, &Error{Pos: pos(template, i), Detail: `unclosed "{"`}
			}
			sub, err := parseSubstitution(template, i+1, i+end, known)
			if err != nil {
				return nil, err
			}
			flush()
			parts = append(parts, sub)
			i += end + 1
		default:
			text.WriteByte(c)
			i++
		}
	}
	flush()
	return &Pattern{parts: parts}, nil
}

func parseSubstitution(template string, start, end int, known func(string) bool) (part, error) {
	fields := strings.Split(template[start:end], "|")
	at := start
	name := fields[0]
	if !nameOK(name) {
		return part{}, &Error{Pos: pos(template, at), Detail: fmt.Sprintf("invalid variable name %q", name)}
	}
	if known != nil && !known(name) {
		return part{}, &Error{UnknownVariable: true, Pos: pos(template, at), Detail: fmt.Sprintf("unknown variable %q", name)}
	}
	out := part{name: name}
	at += len(name) + 1
	for _, f := range fields[1:] {
		switch f {
		case "url":
			out.filters = append(out.filters, filterURL)
		case "raw":
			out.filters = append(out.filters, filterRaw)
		case "lower":
			out.filters = append(out.filters, filterLower)
		default:
			return part{}, &Error{Pos: pos(template, at), Detail: fmt.Sprintf("unknown filter %q (url, raw, lower)", f)}
		}
		at += len(f) + 1
	}
	return out, nil
}

func pos(s string, i int) int { return utf8.RuneCountInString(s[:i]) + 1 }

func nameOK(name string) bool {
	if name == "" {
		return false
	}
	for _, seg := range strings.Split(name, ".") {
		if seg == "" {
			return false
		}
		for i := 0; i < len(seg); i++ {
			if c := seg[i]; (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
				return false
			}
		}
	}
	return true
}

type Values func(name string) (string, bool)

func (p *Pattern) Expand(values Values) (string, []string) {
	var b strings.Builder
	var missing []string
	for _, pt := range p.parts {
		if pt.name == "" {
			b.WriteString(pt.text)
			continue
		}
		v, ok := values(pt.name)
		if !ok {
			if !slices.Contains(missing, pt.name) {
				missing = append(missing, pt.name)
			}
			continue
		}
		raw := false
		for _, f := range pt.filters {
			switch f {
			case filterLower:
				v = strings.ToLower(v)
			case filterRaw:
				raw = true
			}
		}
		if !raw {
			v = Escape(v)
		}
		b.WriteString(v)
	}
	slices.Sort(missing)
	return b.String(), missing
}

func Escape(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&15])
	}
	return b.String()
}
