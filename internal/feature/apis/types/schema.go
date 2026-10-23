package types

import (
	"strings"

	"github.com/pb33f/go-yaml"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
)

type converter struct {
	path     string
	problems []Problem
	deep     bool
}

func refName(ref string) string {
	for _, prefix := range []string{"#/components/schemas/", "#/definitions/"} {
		if name, ok := strings.CutPrefix(ref, prefix); ok {
			return name
		}
	}
	return ref
}

func marker(ext *orderedmap.Map[string, *yaml.Node]) Schema {
	for k, v := range ext.FromOldest() {
		switch k {
		case UnresolvedKey:
			return Schema{"ref": v.Value, "unresolved": true}
		case "x-svcr-ref":
			return Schema{"ref": v.Value}
		}
	}
	return nil
}

func (c *converter) schema(sp *base.SchemaProxy, depth int) Schema {
	if sp == nil {
		return nil
	}
	if sp.IsReference() {
		return Schema{"ref": refName(sp.GetReference())}
	}
	if depth > MaxSchemaDepth {
		if !c.deep {
			c.deep = true
			c.problems = append(c.problems, problem(CodeSchemaTooDeep, c.path, 0, "schema nesting is deeper than 32 levels"))
		}
		return Schema{}
	}
	s := sp.Schema()
	if s == nil {
		return Schema{}
	}
	if m := marker(s.Extensions); m != nil {
		return m
	}
	out := Schema{}
	switch len(s.Type) {
	case 0:
	case 1:
		out["type"] = s.Type[0]
	default:
		out["type"] = s.Type
	}
	setString(out, "format", s.Format)
	setString(out, "description", s.Description)
	if len(s.Enum) > 0 {
		values := make([]any, 0, len(s.Enum))
		for _, n := range s.Enum {
			values = append(values, decode(n))
		}
		out["enum"] = values
	}
	if s.Default != nil {
		out["default"] = decode(s.Default)
	}
	if s.Example != nil {
		out["example"] = decode(s.Example)
	}
	if s.Nullable != nil {
		out["nullable"] = *s.Nullable
	}
	if len(s.Required) > 0 {
		out["required"] = s.Required
	}
	c.children(out, s, depth)
	return out
}

func (c *converter) children(out Schema, s *base.Schema, depth int) {
	if s.Properties != nil {
		props := map[string]any{}
		for name, p := range s.Properties.FromOldest() {
			props[name] = c.schema(p, depth+1)
		}
		out["properties"] = props
	}
	if v := c.dynamic(s.Items, depth); v != nil {
		out["items"] = v
	}
	if v := c.dynamic(s.AdditionalProperties, depth); v != nil {
		out["additional_properties"] = v
	}
	for key, list := range map[string][]*base.SchemaProxy{"one_of": s.OneOf, "any_of": s.AnyOf, "all_of": s.AllOf} {
		if len(list) == 0 {
			continue
		}
		items := make([]Schema, 0, len(list))
		for _, p := range list {
			items = append(items, c.schema(p, depth+1))
		}
		out[key] = items
	}
}

func (c *converter) dynamic(v *base.DynamicValue[*base.SchemaProxy, bool], depth int) any {
	switch {
	case v == nil:
		return nil
	case v.IsA():
		return c.schema(v.A, depth+1)
	}
	return v.B
}

func setString(out Schema, key, v string) {
	if v != "" {
		out[key] = v
	}
}

func decode(n *yaml.Node) any {
	var v any
	if n == nil || n.Decode(&v) != nil {
		return nil
	}
	return v
}
