package main

import "strings"

const module = "svc-registry"

var featureGraph = map[string][]string{
	"access":    {},
	"catalog":   {"access"},
	"ingest":    {"catalog", "access"},
	"links":     {"catalog", "access"},
	"forge":     {"catalog", "access"},
	"deploy":    {"ingest", "forge", "catalog", "access"},
	"knowledge": {"forge", "catalog", "access"},
	"compose":   {"knowledge", "links", "forge", "catalog", "access"},
}

type kind string

const (
	kindNone         kind = ""
	kindRoot         kind = "root"
	kindTypes        kind = "types"
	kindPorts        kind = "ports"
	kindServices     kind = "services"
	kindRepositories kind = "repositories"
	kindAdapter      kind = "adapter"
)

var httpParts = []string{"handlers", "requests", "responses", "middleware", "mcp"}

func under(path, prefix string) bool {
	full := module + "/" + prefix
	return path == full || strings.HasPrefix(path, full+"/")
}

func feature(path string) string {
	rest, ok := strings.CutPrefix(path, module+"/internal/feature/")
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(rest, "/")
	return name
}

func kindOf(path string) kind {
	rest, ok := strings.CutPrefix(path, module+"/internal/feature/")
	if !ok {
		return kindNone
	}
	_, sub, found := strings.Cut(rest, "/")
	if !found {
		return kindRoot
	}
	first, _, _ := strings.Cut(sub, "/")
	switch kind(first) {
	case kindTypes, kindPorts, kindServices, kindRepositories:
		return kind(first)
	}
	return kindAdapter
}

func allowed(from, to string) bool {
	for _, dep := range featureGraph[from] {
		if dep == to {
			return true
		}
	}
	return false
}

func httpPart(path string) bool {
	for _, part := range httpParts {
		if under(path, "internal/presentation/http/"+part) {
			return true
		}
	}
	return false
}

func fiber(path string) bool { return strings.HasPrefix(path, "github.com/gofiber/") }
