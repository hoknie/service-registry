package mcp

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"svc-registry/internal/feature/catalog"
	knowledgeservice "svc-registry/internal/feature/knowledge/service"
)

const (
	docsTemplate     = "svcr://projects/{project}/docs/{+path}"
	resourcePageSize = 200
)

func docURI(project uuid.UUID, path string) string {
	return "svcr://projects/" + project.String() + "/docs/" + path
}

func mimeOf(path string) string {
	if strings.HasSuffix(strings.ToLower(path), ".md") {
		return "text/markdown"
	}
	return "text/plain"
}

func (t *tools) registerResources(s *sdk.Server) {
	s.AddResourceTemplate(&sdk.ResourceTemplate{Name: "docs", Title: "Project documentation",
		URITemplate: docsTemplate, Description: "A documentation file of a project; add ?ref=<branch> for another branch." + dataNote},
		t.readResource)
	s.AddReceivingMiddleware(func(next sdk.MethodHandler) sdk.MethodHandler {
		return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
			if method != "resources/list" {
				return next(ctx, method, req)
			}
			cursor := ""
			if params, ok := req.GetParams().(*sdk.ListResourcesParams); ok && params != nil {
				cursor = params.Cursor
			}
			return t.listResources(ctx, cursor)
		}
	})
}

func (t *tools) readResource(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	uri := req.Params.URI
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "svcr" || u.Host != "projects" {
		return nil, sdk.ResourceNotFoundError(uri)
	}
	project, path, ok := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/docs/")
	if !ok || project == "" || path == "" {
		return nil, sdk.ResourceNotFoundError(uri)
	}
	project, _ = url.PathUnescape(project)
	s, err := resolveProject(ctx, t.deps, t.p, project)
	if err != nil {
		return nil, sdk.ResourceNotFoundError(uri)
	}
	ref := knowledgeservice.KnowledgeRef{Branch: opt(u.Query().Get("ref"))}
	_, f, err := t.deps.Knowledge.KnowledgeFile(ctx, t.p, s.Node.ID, ref, path)
	if err != nil || f.Content == nil {
		return nil, sdk.ResourceNotFoundError(uri)
	}
	return &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{{URI: uri, MIMEType: mimeOf(path), Text: *f.Content}}}, nil
}

func (t *tools) listResources(ctx context.Context, cursor string) (*sdk.ListResourcesResult, error) {
	offset, _ := strconv.Atoi(cursor)
	depth := int64(catalog.TreeMaxDepth)
	nodes, _, err := t.deps.Catalog.Tree(ctx, t.p, nil, &depth)
	if err != nil {
		return &sdk.ListResourcesResult{Resources: []*sdk.Resource{}}, nil
	}
	var all []*sdk.Resource
	for _, w := range nodes {
		if !w.Readable || w.Node.Kind != catalog.KindProject {
			continue
		}
		_, files, err := t.deps.Knowledge.KnowledgeFiles(ctx, t.p, w.Node.ID, knowledgeservice.KnowledgeRef{})
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.Skip != nil {
				continue
			}
			all = append(all, &sdk.Resource{URI: docURI(w.Node.ID, f.Path), Name: w.Node.Slug + "/" + f.Path,
				Title: w.Node.Name + ": " + f.Path, MIMEType: mimeOf(f.Path), Size: f.Bytes})
		}
	}
	out := &sdk.ListResourcesResult{Resources: []*sdk.Resource{}}
	if offset < len(all) {
		out.Resources = all[offset:min(offset+resourcePageSize, len(all))]
	}
	if offset+resourcePageSize < len(all) {
		out.NextCursor = strconv.Itoa(offset + resourcePageSize)
	}
	return out, nil
}
