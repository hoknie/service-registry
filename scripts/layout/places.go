package main

import "strings"

const module = "svc-registry"

var featureGraph = map[string][]string{
	"access":     {},
	"catalog":    {"access"},
	"ownership":  {"catalog", "access"},
	"notify":     {"ownership", "catalog", "access"},
	"topology":   {"catalog", "access"},
	"ingest":     {"notify", "topology", "catalog", "access"},
	"links":      {"notify", "catalog", "access"},
	"reposync":   {"notify", "catalog", "access"},
	"deploy":     {"ingest", "reposync", "notify", "topology", "catalog", "access"},
	"knowledge":  {"reposync", "topology", "catalog", "access"},
	"apis":       {"knowledge", "catalog", "access"},
	"publishing": {"apis", "knowledge", "catalog", "access"},
	"compose":    {"knowledge", "links", "ownership", "reposync", "catalog", "access"},
	"skills":     {"knowledge", "catalog", "access"},
	"gateway":    {"catalog", "access"},
	"membership": {"catalog", "access"},
}

type kind string

const (
	kindNone         kind = ""
	kindRoot         kind = "root"
	kindTypes        kind = "types"
	kindPorts        kind = "ports"
	kindServices     kind = "services"
	kindRepositories kind = "repositories"
	kindUseCases     kind = "usecases"
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
	case kindTypes, kindPorts, kindServices, kindRepositories, kindUseCases:
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
