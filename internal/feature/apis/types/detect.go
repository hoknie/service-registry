package types

import (
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"go.yaml.in/yaml/v3"
)

type Candidate struct {
	Path    string
	Skipped bool
}

func IsCandidate(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".proto", ".yaml", ".yml", ".json":
		return true
	}
	return false
}

func IsProto(p string) bool { return strings.EqualFold(path.Ext(p), ".proto") }

var vendoredDirs = []string{"**/third_party/**", "**/vendor/**", "**/node_modules/**"}

var vendoredProtos = []string{"**/google/**", "**/buf/validate/**", "**/validate/validate.proto", "**/protoc-gen-openapiv2/**",
	"**/gogoproto/**"}

func Vendored(p string) bool {
	patterns := vendoredDirs
	if IsProto(p) {
		patterns = append(append([]string{}, vendoredDirs...), vendoredProtos...)
	}
	for _, pattern := range patterns {
		if ok, _ := doublestar.Match(pattern, p); ok {
			return true
		}
	}
	return false
}

func DetectSkipped(p string) (Format, bool) {
	if IsProto(p) {
		return FormatProto, true
	}
	name := strings.ToLower(path.Base(p))
	if !IsCandidate(p) || !(strings.HasPrefix(name, "openapi") || strings.HasPrefix(name, "swagger")) {
		return "", false
	}
	return FormatOpenAPI, true
}

func Detect(p, content string) (Format, string, bool) {
	if IsProto(p) {
		return FormatProto, "", true
	}
	if !IsCandidate(p) {
		return "", "", false
	}
	var head struct {
		OpenAPI any `yaml:"openapi"`
		Swagger any `yaml:"swagger"`
	}
	if err := yaml.Unmarshal([]byte(content), &head); err != nil {
		return "", "", false
	}
	if v, ok := head.OpenAPI.(string); ok && strings.HasPrefix(v, "3.") {
		return FormatOpenAPI, v, true
	}
	if v, ok := head.Swagger.(string); ok && v == "2.0" {
		return FormatSwagger, v, true
	}
	return "", "", false
}
