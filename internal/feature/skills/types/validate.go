package types

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MaxFolderDepth = 8
	MaxAbout       = 4000
	MaxImageBytes  = 512 << 10
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$`)

func ValidSlug(s string) bool { return slugPattern.MatchString(s) }

func CleanFolder(slug, name, description string) (string, string, string, error) {
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if !ValidSlug(slug) || name == "" || utf8.RuneCountInString(name) > 200 || utf8.RuneCountInString(description) > 2000 {
		return "", "", "", InvalidFolder
	}
	return slug, name, description, nil
}

func CleanRoot(raw string) (string, error) {
	raw = strings.Trim(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", nil
	}
	return CleanPath(raw)
}
