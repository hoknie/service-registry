package types

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v3"

	"svc-registry/pkg/ptr"
)

var lineInError = regexp.MustCompile(`line (\d+)`)

func emptyOpenAPI() *OpenAPI {
	return &OpenAPI{Servers: []Server{}, Tags: []Tag{}, Operations: []Operation{}, Schemas: []NamedSchema{}}
}

func ParseOpenAPI(p, content string, format Format, version string, files *Files) (Document, error) {
	doc := Document{Path: p, Format: format, SpecVersion: ptr.NonEmpty(version), Problems: []Problem{}, OpenAPI: emptyOpenAPI()}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		doc.Problems = append(doc.Problems, invalid(p, err))
		return doc, nil
	}
	if bad := structureProblem(p, &root); bad != nil {
		doc.Problems = append(doc.Problems, *bad)
		return doc, nil
	}
	bundled, problems, err := Bundle(p, &root, files)
	if err != nil {
		return Document{}, err
	}
	doc.Problems = append(doc.Problems, problems...)
	if bundled == nil {
		bundled = []byte(content)
	}
	c := &converter{path: p}
	model, perr := build(bundled, format, c)
	if perr != nil {
		doc.Problems = append(doc.Problems, invalid(p, perr))
		return doc, nil
	}
	doc.OpenAPI = model
	doc.Problems = append(doc.Problems, c.problems...)
	if len(model.Operations) > MaxItems {
		model.Operations = model.Operations[:MaxItems]
		doc.Problems = append(doc.Problems, problem(CodeTooLarge, p, 0, "more than 5000 operations; the first 5000 are shown"))
	}
	return doc, nil
}

func invalid(p string, err error) Problem {
	line := 0
	if m := lineInError.FindStringSubmatch(err.Error()); m != nil {
		line, _ = strconv.Atoi(m[1])
	}
	return problem(CodeInvalidDocument, p, line, err.Error())
}

func structureProblem(p string, root *yaml.Node) *Problem {
	top := root
	if top.Kind == yaml.DocumentNode && len(top.Content) == 1 {
		top = top.Content[0]
	}
	if top.Kind != yaml.MappingNode {
		pr := problem(CodeInvalidDocument, p, top.Line, "the document is not an object")
		return &pr
	}
	for _, key := range []string{"info", "paths", "components", "definitions"} {
		if v := child(top, key); v != nil && v.Kind != yaml.MappingNode {
			pr := problem(CodeInvalidDocument, p, v.Line, fmt.Sprintf("%q must be an object", key))
			return &pr
		}
	}
	return nil
}

func build(content []byte, format Format, c *converter) (model *OpenAPI, err error) {
	defer func() {
		if r := recover(); r != nil {
			model, err = nil, fmt.Errorf("the document cannot be read: %v", r)
		}
	}()
	d, err := libopenapi.NewDocumentWithConfiguration(content, &datamodel.DocumentConfiguration{
		Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		return nil, err
	}
	if format == FormatSwagger {
		m, err := d.BuildV2Model()
		if m == nil {
			return nil, err
		}
		return c.swagger(m.Model), nil
	}
	m, err := d.BuildV3Model()
	if m == nil {
		return nil, err
	}
	return c.openapi(m.Model), nil
}

func (c *converter) openapi(d v3.Document) *OpenAPI {
	out := emptyOpenAPI()
	if d.Info != nil {
		out.Info = Info{Title: d.Info.Title, Version: d.Info.Version, Description: ptr.NonEmpty(d.Info.Description)}
	}
	for _, s := range d.Servers {
		out.Servers = append(out.Servers, Server{URL: s.URL, Description: ptr.NonEmpty(s.Description)})
	}
	for _, t := range d.Tags {
		out.Tags = append(out.Tags, Tag{Name: t.Name, Description: ptr.NonEmpty(t.Description)})
	}
	if d.Paths != nil {
		for path, item := range d.Paths.PathItems.FromOldest() {
			for _, mo := range []struct {
				m  string
				op *v3.Operation
			}{{"GET", item.Get}, {"PUT", item.Put}, {"POST", item.Post}, {"DELETE", item.Delete},
				{"OPTIONS", item.Options}, {"HEAD", item.Head}, {"PATCH", item.Patch}, {"TRACE", item.Trace}} {
				if mo.op != nil {
					out.Operations = append(out.Operations, c.operation(mo.m, path, mo.op, item.Parameters))
				}
			}
		}
	}
	if d.Components != nil {
		for name, sp := range d.Components.Schemas.FromOldest() {
			out.Schemas = append(out.Schemas, NamedSchema{Name: name, Schema: c.schema(sp, 0)})
		}
	}
	finish(out)
	return out
}

func (c *converter) operation(method, path string, op *v3.Operation, shared []*v3.Parameter) Operation {
	o := Operation{Key: method + " " + path, Method: method, Path: path, OperationID: ptr.NonEmpty(op.OperationId),
		Summary: ptr.NonEmpty(op.Summary), Description: ptr.NonEmpty(op.Description), Tags: op.Tags,
		Deprecated: ptr.Value(op.Deprecated), Parameters: []Parameter{}, Responses: []Response{}}
	if o.Tags == nil {
		o.Tags = []string{}
	}
	for _, prm := range mergeParams(shared, op.Parameters) {
		o.Parameters = append(o.Parameters, Parameter{Name: prm.Name, In: prm.In, Required: ptr.Value(prm.Required),
			Description: ptr.NonEmpty(prm.Description), Schema: c.schema(prm.Schema, 0)})
	}
	if rb := op.RequestBody; rb != nil {
		o.RequestBody = &RequestBody{Required: ptr.Value(rb.Required), Description: ptr.NonEmpty(rb.Description), Content: c.content(rb.Content)}
	}
	if op.Responses != nil {
		for code, r := range op.Responses.Codes.FromOldest() {
			o.Responses = append(o.Responses, Response{Status: code, Description: ptr.NonEmpty(r.Description), Content: c.content(r.Content)})
		}
		if r := op.Responses.Default; r != nil {
			o.Responses = append(o.Responses, Response{Status: "default", Description: ptr.NonEmpty(r.Description), Content: c.content(r.Content)})
		}
	}
	return o
}

func mergeParams(shared, own []*v3.Parameter) []*v3.Parameter {
	out := []*v3.Parameter{}
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

func (c *converter) content(m *orderedmap.Map[string, *v3.MediaType]) []Content {
	out := []Content{}
	for mt, v := range m.FromOldest() {
		out = append(out, Content{MediaType: mt, Schema: c.schema(v.Schema, 0)})
	}
	return out
}

func finish(out *OpenAPI) {
	known := map[string]bool{}
	for _, t := range out.Tags {
		known[t.Name] = true
	}
	for _, op := range out.Operations {
		for _, t := range op.Tags {
			if !known[t] {
				known[t] = true
				out.Tags = append(out.Tags, Tag{Name: t})
			}
		}
	}
}
