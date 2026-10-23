package types

import (
	v2 "github.com/pb33f/libopenapi/datamodel/high/v2"

	"svc-registry/pkg/ptr"
)

const defaultMedia = "application/json"

func (c *converter) swagger(d v2.Swagger) *OpenAPI {
	out := emptyOpenAPI()
	if d.Info != nil {
		out.Info = Info{Title: d.Info.Title, Version: d.Info.Version, Description: ptr.NonEmpty(d.Info.Description)}
	}
	out.Servers = swaggerServers(d)
	for _, t := range d.Tags {
		out.Tags = append(out.Tags, Tag{Name: t.Name, Description: ptr.NonEmpty(t.Description)})
	}
	if d.Paths != nil {
		for path, item := range d.Paths.PathItems.FromOldest() {
			for _, mo := range []struct {
				m  string
				op *v2.Operation
			}{{"GET", item.Get}, {"PUT", item.Put}, {"POST", item.Post}, {"DELETE", item.Delete},
				{"OPTIONS", item.Options}, {"HEAD", item.Head}, {"PATCH", item.Patch}} {
				if mo.op != nil {
					out.Operations = append(out.Operations, c.swaggerOperation(d, mo.m, path, mo.op, item.Parameters))
				}
			}
		}
	}
	if d.Definitions != nil {
		for name, sp := range d.Definitions.Definitions.FromOldest() {
			out.Schemas = append(out.Schemas, NamedSchema{Name: name, Schema: c.schema(sp, 0)})
		}
	}
	finish(out)
	return out
}

func swaggerServers(d v2.Swagger) []Server {
	out := []Server{}
	if d.Host == "" {
		if d.BasePath != "" {
			out = append(out, Server{URL: d.BasePath})
		}
		return out
	}
	schemes := d.Schemes
	if len(schemes) == 0 {
		schemes = []string{"https"}
	}
	for _, s := range schemes {
		out = append(out, Server{URL: s + "://" + d.Host + d.BasePath})
	}
	return out
}

func (c *converter) swaggerOperation(d v2.Swagger, method, path string, op *v2.Operation, shared []*v2.Parameter) Operation {
	o := Operation{Key: method + " " + path, Method: method, Path: path, OperationID: ptr.NonEmpty(op.OperationId),
		Summary: ptr.NonEmpty(op.Summary), Description: ptr.NonEmpty(op.Description), Tags: op.Tags,
		Deprecated: op.Deprecated, Parameters: []Parameter{}, Responses: []Response{}}
	if o.Tags == nil {
		o.Tags = []string{}
	}
	consumes := orDefault(op.Consumes, d.Consumes)
	for _, prm := range mergeSwaggerParams(shared, op.Parameters) {
		if prm.In == "body" {
			rb := &RequestBody{Required: ptr.Value(prm.Required), Description: ptr.NonEmpty(prm.Description), Content: []Content{}}
			for _, mt := range consumes {
				rb.Content = append(rb.Content, Content{MediaType: mt, Schema: c.schema(prm.Schema, 0)})
			}
			o.RequestBody = rb
			continue
		}
		o.Parameters = append(o.Parameters, Parameter{Name: prm.Name, In: prm.In, Required: ptr.Value(prm.Required),
			Description: ptr.NonEmpty(prm.Description), Schema: simpleSchema(prm)})
	}
	produces := orDefault(op.Produces, d.Produces)
	if op.Responses != nil {
		for code, r := range op.Responses.Codes.FromOldest() {
			o.Responses = append(o.Responses, c.swaggerResponse(code, r, produces))
		}
		if r := op.Responses.Default; r != nil {
			o.Responses = append(o.Responses, c.swaggerResponse("default", r, produces))
		}
	}
	return o
}

func (c *converter) swaggerResponse(code string, r *v2.Response, produces []string) Response {
	out := Response{Status: code, Description: ptr.NonEmpty(r.Description), Content: []Content{}}
	if r.Schema != nil {
		for _, mt := range produces {
			out.Content = append(out.Content, Content{MediaType: mt, Schema: c.schema(r.Schema, 0)})
		}
	}
	return out
}

func orDefault(own, global []string) []string {
	switch {
	case len(own) > 0:
		return own
	case len(global) > 0:
		return global
	}
	return []string{defaultMedia}
}

func mergeSwaggerParams(shared, own []*v2.Parameter) []*v2.Parameter {
	out := []*v2.Parameter{}
	seen := map[string]bool{}
	for _, p := range own {
		if p != nil {
			seen[p.In+"\x00"+p.Name] = true
			out = append(out, p)
		}
	}
	for _, p := range shared {
		if p != nil && !seen[p.In+"\x00"+p.Name] {
			out = append(out, p)
		}
	}
	return out
}

func simpleSchema(p *v2.Parameter) Schema {
	out := Schema{}
	if m := marker(p.Extensions); m != nil {
		return m
	}
	setString(out, "type", p.Type)
	setString(out, "format", p.Format)
	if len(p.Enum) > 0 {
		values := make([]any, 0, len(p.Enum))
		for _, n := range p.Enum {
			values = append(values, decode(n))
		}
		out["enum"] = values
	}
	if p.Default != nil {
		out["default"] = decode(p.Default)
	}
	if p.Items != nil {
		items := Schema{}
		setString(items, "type", p.Items.Type)
		setString(items, "format", p.Items.Format)
		out["items"] = items
	}
	return out
}
