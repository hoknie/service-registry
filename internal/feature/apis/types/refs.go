package types

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const UnresolvedKey = "x-svcr-unresolved"

type bundler struct {
	root     string
	files    *Files
	docs     map[string]*yaml.Node
	problems []Problem
	changed  bool
}

func Bundle(rootPath string, root *yaml.Node, files *Files) ([]byte, []Problem, error) {
	b := &bundler{root: rootPath, files: files, docs: map[string]*yaml.Node{rootPath: root}}
	if err := b.walk(root, rootPath, 0, map[string]bool{}); err != nil {
		return nil, nil, err
	}
	if !b.changed {
		return nil, b.problems, nil
	}
	out, err := yaml.Marshal(root)
	return out, b.problems, err
}

func (b *bundler) walk(n *yaml.Node, file string, depth int, stack map[string]bool) error {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			if err := b.walk(c, file, depth, stack); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		if ref := refValue(n); ref != nil {
			return b.replace(n, ref, file, depth, stack)
		}
		for i := 1; i < len(n.Content); i += 2 {
			if err := b.walk(n.Content[i], file, depth, stack); err != nil {
				return err
			}
		}
	}
	return nil
}

func refValue(n *yaml.Node) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "$ref" && n.Content[i+1].Kind == yaml.ScalarNode {
			return n.Content[i+1]
		}
	}
	return nil
}

func (b *bundler) replace(n, ref *yaml.Node, file string, depth int, stack map[string]bool) error {
	target, fragment, _ := strings.Cut(ref.Value, "#")
	targetFile := file
	if target != "" {
		p, ok := ResolveRef(file, target)
		if !ok {
			b.unresolved(n, ref, file)
			return nil
		}
		targetFile = p
	}
	doc, err := b.doc(targetFile)
	if err != nil {
		return err
	}
	found := pointer(doc, fragment)
	if found == nil {
		b.unresolved(n, ref, file)
		return nil
	}
	if targetFile == b.root {
		if file != b.root {
			ref.Value = "#" + fragment
			b.changed = true
		}
		return nil
	}
	key := targetFile + "#" + fragment
	if stack[key] || depth >= MaxSchemaDepth {
		name := fragment[strings.LastIndex(fragment, "/")+1:]
		*n = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar("x-svcr-ref"), scalar(name)}}
		b.changed = true
		return nil
	}
	cp := deepCopy(found)
	stack[key] = true
	err = b.walk(cp, targetFile, depth+1, stack)
	delete(stack, key)
	*n = *cp
	b.changed = true
	return err
}

func (b *bundler) unresolved(n, ref *yaml.Node, file string) {
	b.problems = append(b.problems, problem(CodeRefUnresolved, file, ref.Line, fmt.Sprintf("reference %q is not found in the snapshot", ref.Value)))
	*n = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar(UnresolvedKey), scalar(ref.Value)}}
	b.changed = true
}

func (b *bundler) doc(p string) (*yaml.Node, error) {
	if d, ok := b.docs[p]; ok {
		return d, nil
	}
	content, ok, err := b.files.Read(p)
	if err != nil {
		return nil, err
	}
	var d yaml.Node
	if !ok || yaml.Unmarshal([]byte(content), &d) != nil {
		b.docs[p] = nil
		return nil, nil
	}
	b.docs[p] = &d
	return &d, nil
}

func pointer(doc *yaml.Node, fragment string) *yaml.Node {
	if doc == nil {
		return nil
	}
	n := doc
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	if fragment == "" || fragment == "/" {
		return n
	}
	if !strings.HasPrefix(fragment, "/") {
		return nil
	}
	for _, raw := range strings.Split(fragment[1:], "/") {
		part, err := url.PathUnescape(raw)
		if err != nil {
			return nil
		}
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		if n = child(n, part); n == nil {
			return nil
		}
	}
	return n
}

func child(n *yaml.Node, part string) *yaml.Node {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == part {
				return n.Content[i+1]
			}
		}
	case yaml.SequenceNode:
		if i, err := strconv.Atoi(part); err == nil && i >= 0 && i < len(n.Content) {
			return n.Content[i]
		}
	}
	return nil
}

func deepCopy(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.AliasNode {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	}
	cp := *n
	cp.Content = make([]*yaml.Node, len(n.Content))
	for i, c := range n.Content {
		cp.Content[i] = deepCopy(c)
	}
	return &cp
}

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}
