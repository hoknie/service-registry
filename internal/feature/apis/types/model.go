package types

import "svc-registry/pkg/ptr"

type Format string

const (
	FormatOpenAPI Format = "openapi"
	FormatSwagger Format = "swagger"
	FormatProto   Format = "proto"
)

const (
	CodeInvalidDocument = "api.invalid_document"
	CodeRefUnresolved   = "api.ref_unresolved"
	CodeImportNotFound  = "api.import_not_found"
	CodeSchemaTooDeep   = "api.schema_too_deep"
	CodeFileSkipped     = "api.file_skipped"
	CodeTooLarge        = "api.too_large"
)

const (
	MaxItems       = 5000
	MaxSchemaDepth = 32
)

type Problem struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Line    *int   `json:"line"`
	Message string `json:"message"`
}

type Summary struct {
	Path        string  `json:"path"`
	Format      Format  `json:"format"`
	SpecVersion *string `json:"spec_version"`
	Title       *string `json:"title"`
	Version     *string `json:"version"`
	Operations  *int    `json:"operations"`
	Services    *int    `json:"services"`
	Methods     *int    `json:"methods"`
	Messages    *int    `json:"messages"`
	SkipReason  *string `json:"skip_reason"`
	Problems    int     `json:"problems"`
	Vendored    bool    `json:"vendored"`
}

type Document struct {
	Path        string    `json:"path"`
	Format      Format    `json:"format"`
	SpecVersion *string   `json:"spec_version"`
	Problems    []Problem `json:"problems"`
	*OpenAPI
	*Proto
	skip *string
}

type Info struct {
	Title       string  `json:"title"`
	Version     string  `json:"version"`
	Description *string `json:"description"`
}

type Server struct {
	URL         string  `json:"url"`
	Description *string `json:"description"`
}

type Tag struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type OpenAPI struct {
	Info       Info          `json:"info"`
	Servers    []Server      `json:"servers"`
	Tags       []Tag         `json:"tags"`
	Operations []Operation   `json:"operations"`
	Schemas    []NamedSchema `json:"schemas"`
}

type Operation struct {
	Key         string       `json:"key"`
	Method      string       `json:"method"`
	Path        string       `json:"path"`
	OperationID *string      `json:"operation_id"`
	Summary     *string      `json:"summary"`
	Description *string      `json:"description"`
	Tags        []string     `json:"tags"`
	Deprecated  bool         `json:"deprecated"`
	Parameters  []Parameter  `json:"parameters"`
	RequestBody *RequestBody `json:"request_body"`
	Responses   []Response   `json:"responses"`
}

type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Required    bool    `json:"required"`
	Description *string `json:"description"`
	Schema      Schema  `json:"schema"`
}

type Content struct {
	MediaType string `json:"media_type"`
	Schema    Schema `json:"schema"`
}

type RequestBody struct {
	Required    bool      `json:"required"`
	Description *string   `json:"description"`
	Content     []Content `json:"content"`
}

type Response struct {
	Status      string    `json:"status"`
	Description *string   `json:"description"`
	Content     []Content `json:"content"`
}

type NamedSchema struct {
	Name   string `json:"name"`
	Schema Schema `json:"schema"`
}

type Schema map[string]any

type Proto struct {
	Package   *string           `json:"package"`
	Imports   []string          `json:"imports"`
	Options   map[string]string `json:"options"`
	Services  []Service         `json:"services"`
	Messages  []Message         `json:"messages"`
	Enums     []Enum            `json:"enums"`
	TypeFiles map[string]string `json:"type_files"`
}

type Service struct {
	Name        string   `json:"name"`
	FullName    string   `json:"full_name"`
	Description *string  `json:"description"`
	Methods     []Method `json:"methods"`
}

type HTTPRule struct {
	Method string  `json:"method"`
	Path   string  `json:"path"`
	Body   *string `json:"body"`
}

type Method struct {
	Name            string    `json:"name"`
	Input           string    `json:"input"`
	Output          string    `json:"output"`
	ClientStreaming bool      `json:"client_streaming"`
	ServerStreaming bool      `json:"server_streaming"`
	Description     *string   `json:"description"`
	Deprecated      bool      `json:"deprecated"`
	HTTP            *HTTPRule `json:"http"`
}

type MapType struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Field struct {
	Name        string   `json:"name"`
	Number      int32    `json:"number"`
	Type        string   `json:"type"`
	Label       *string  `json:"label"`
	Map         *MapType `json:"map"`
	Oneof       *string  `json:"oneof"`
	Description *string  `json:"description"`
	Deprecated  bool     `json:"deprecated"`
}

type Message struct {
	FullName    string  `json:"full_name"`
	Description *string `json:"description"`
	Deprecated  bool    `json:"deprecated"`
	Fields      []Field `json:"fields"`
}

type EnumValue struct {
	Name        string  `json:"name"`
	Number      int32   `json:"number"`
	Description *string `json:"description"`
}

type Enum struct {
	FullName    string      `json:"full_name"`
	Description *string     `json:"description"`
	Values      []EnumValue `json:"values"`
}

func (d Document) Summary() Summary {
	s := Summary{Path: d.Path, Format: d.Format, SpecVersion: d.SpecVersion, Problems: len(d.Problems), SkipReason: d.skip,
		Vendored: Vendored(d.Path)}
	switch {
	case d.skip != nil:
	case d.OpenAPI != nil:
		s.Title, s.Version = ptr.NonEmpty(d.OpenAPI.Info.Title), ptr.NonEmpty(d.OpenAPI.Info.Version)
		s.Operations = new(len(d.OpenAPI.Operations))
	case d.Proto != nil:
		s.Title = d.Proto.Package
		methods := 0
		for _, sv := range d.Proto.Services {
			methods += len(sv.Methods)
		}
		s.Services, s.Methods, s.Messages = new(len(d.Proto.Services)), new(methods), new(len(d.Proto.Messages))
	}
	return s
}

func problem(code, path string, line int, msg string) Problem {
	p := Problem{Code: code, Path: path, Message: msg}
	if line > 0 {
		p.Line = &line
	}
	return p
}

func SkippedDocument(p string, format Format, reason string) Document {
	d := Document{Path: p, Format: format, Problems: []Problem{problem(CodeFileSkipped, p, 0, "the file was skipped during collection: "+reason)}}
	if format == FormatProto {
		d.Proto = emptyProto()
	} else {
		d.OpenAPI = emptyOpenAPI()
	}
	d.skip = &reason
	return d
}
