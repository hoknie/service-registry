package knowledge

import (
	"regexp"
	"strconv"
	"strings"
)

type Kind string

const (
	KindDoc    Kind = "doc"
	KindSpec   Kind = "spec"
	KindChange Kind = "change"
	KindADR    Kind = "adr"
)

type Meta struct {
	Capability   string   `json:"capability,omitempty"`
	Purpose      string   `json:"purpose,omitempty"`
	Requirements []string `json:"requirements,omitempty"`
	Change       string   `json:"change,omitempty"`
	Archived     bool     `json:"archived,omitempty"`
	Number       int      `json:"number,omitempty"`
	Title        string   `json:"title,omitempty"`
	Status       string   `json:"status,omitempty"`
	Supersedes   string   `json:"supersedes,omitempty"`
}

var adrName = regexp.MustCompile(`^openspec/decisions/(\d{4,})-[^/]+\.md$`)

func Classify(p string) (Kind, Meta) {
	switch {
	case strings.HasPrefix(p, "openspec/specs/") && strings.HasSuffix(p, "/spec.md"):
		capability := strings.TrimSuffix(strings.TrimPrefix(p, "openspec/specs/"), "/spec.md")
		if capability != "" {
			return KindSpec, Meta{Capability: capability}
		}
	case strings.HasPrefix(p, "openspec/changes/archive/"):
		if id, _, ok := strings.Cut(strings.TrimPrefix(p, "openspec/changes/archive/"), "/"); ok && id != "" {
			return KindChange, Meta{Change: id, Archived: true}
		}
	case strings.HasPrefix(p, "openspec/changes/"):
		if id, _, ok := strings.Cut(strings.TrimPrefix(p, "openspec/changes/"), "/"); ok && id != "" && id != "archive" {
			return KindChange, Meta{Change: id}
		}
	case adrName.MatchString(p):
		n, _ := strconv.Atoi(adrName.FindStringSubmatch(p)[1])
		return KindADR, Meta{Number: n}
	}
	return KindDoc, Meta{}
}

func Parse(kind Kind, _ string, content string, meta Meta) Meta {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	switch kind {
	case KindSpec:
		var purpose []string
		inPurpose := false
		for _, l := range lines {
			switch {
			case strings.HasPrefix(l, "## "):
				inPurpose = strings.TrimSpace(strings.TrimPrefix(l, "## ")) == "Purpose"
			case strings.HasPrefix(l, "### Requirement:"):
				inPurpose = false
				meta.Requirements = append(meta.Requirements, strings.TrimSpace(strings.TrimPrefix(l, "### Requirement:")))
			case inPurpose && !strings.HasPrefix(l, "#"):
				purpose = append(purpose, l)
			}
		}
		meta.Purpose = strings.TrimSpace(strings.Join(purpose, "\n"))
	case KindADR:
		for _, l := range lines {
			if meta.Title == "" && strings.HasPrefix(l, "# ") {
				meta.Title = strings.TrimSpace(strings.TrimPrefix(l, "# "))
				continue
			}
			field, value, ok := strings.Cut(strings.ReplaceAll(strings.TrimLeft(l, "-* \t"), "*", ""), ":")
			if !ok {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(field)) {
			case "status":
				if meta.Status == "" {
					meta.Status = strings.TrimSpace(value)
				}
			case "supersedes":
				if meta.Supersedes == "" {
					meta.Supersedes = strings.TrimSpace(value)
				}
			}
		}
	}
	return meta
}
