package webui

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	errEncoding  = errors.New("bad path: encoding")
	errEmpty     = errors.New("bad path: empty segment")
	errDot       = errors.New("bad path: dot segment")
	errSeparator = errors.New("bad path: separator or control character")
)

func parsePath(raw string) ([]string, error) {
	rest := strings.TrimPrefix(raw, "/")
	if rest == "" {
		return nil, nil
	}
	parts := strings.Split(rest, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		seg, err := segment(p)
		if err != nil {
			return nil, err
		}
		out = append(out, seg)
	}
	return out, nil
}

func segment(raw string) (string, error) {
	if raw == "" {
		return "", errEmpty
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil || !utf8.ValidString(decoded) {
		return "", errEncoding
	}
	if decoded == "." || decoded == ".." {
		return "", errDot
	}
	for _, r := range decoded {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			return "", errSeparator
		}
	}
	return decoded, nil
}
