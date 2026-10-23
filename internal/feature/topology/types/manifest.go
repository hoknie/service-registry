package types

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v3"
)

const MaxManifestBytes = 64 << 10

var ManifestNames = []string{".svc-registry.yaml", ".svc-registry.yml"}

type Manifest struct {
	Refs    []Ref
	Problem string
}

func ParseManifest(data []byte) (Manifest, error) {
	if len(data) > MaxManifestBytes {
		return Manifest{}, errors.New("the file is larger than 64 KiB")
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Manifest{}, errors.New("the file is not a YAML object")
	}
	raw, ok := doc["dependencies"]
	if !ok || raw == nil {
		return Manifest{}, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return Manifest{}, errors.New("dependencies is not a list")
	}
	var m Manifest
	var texts []string
	bad := 0
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			bad++
			continue
		}
		texts = append(texts, s)
	}
	if len(texts) > MaxRefs {
		m.Problem = fmt.Sprintf("only the first %d dependencies are used", MaxRefs)
		texts = texts[:MaxRefs]
	}
	refs, invalid := ParseRefs(texts)
	m.Refs = refs
	if bad+invalid > 0 {
		m.Problem = fmt.Sprintf("%d dependencies are not valid references", bad+invalid)
	}
	return m, nil
}
