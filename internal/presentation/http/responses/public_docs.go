package responses

import (
	"strings"
	apistypes "svc-registry/internal/feature/apis/types"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	publishingservices "svc-registry/internal/feature/publishing/services"
	publishingtypes "svc-registry/internal/feature/publishing/types"
	"svc-registry/pkg/ptr"
)

type PublicEntry struct {
	Name  string  `json:"name"`
	Title string  `json:"title"`
	Kind  string  `json:"kind"`
	Host  *string `json:"host"`
}

type PublicNode struct {
	Kind        string  `json:"kind"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	HasDocs     *bool   `json:"has_docs,omitempty"`
}

type PublicVersion struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	CollectedAt string `json:"collected_at"`
	Release     bool   `json:"release"`
}

type PublicDocs struct {
	Default  *PublicVersion  `json:"default"`
	Branches []PublicVersion `json:"branches"`
	Tags     []PublicVersion `json:"tags"`
	Release  *string         `json:"release"`
}

type PublicSpace struct {
	Entry     PublicEntry        `json:"entry"`
	Projects  []PublicHitProject `json:"projects"`
	Node      PublicNode         `json:"node"`
	Ancestors []PublicNode       `json:"ancestors"`
	Children  []PublicNode       `json:"children"`
	Docs      *PublicDocs        `json:"docs"`
	DocPath   *string            `json:"doc_path"`
}

func publicNode(n publishingtypes.Node) PublicNode {
	return PublicNode{Kind: n.Kind, Slug: n.Slug, Name: n.Name}
}

func publicVersion(v publishingtypes.PublicVersion) PublicVersion {
	return PublicVersion{Name: v.Name, Kind: string(v.Kind), CollectedAt: v.CollectedAt, Release: v.Release}
}

func PublicSpaceOf(s publishingservices.PublicSpace, segment string) PublicSpace {
	node := publicNode(s.Node)
	node.Description = new(s.Node.Description)
	out := PublicSpace{Entry: PublicEntry{Name: segment, Title: s.Entry.Name, Kind: s.Entry.Kind, Host: s.EntryHost}, Node: node,
		Ancestors: make([]PublicNode, 0, len(s.Ancestors)), Children: make([]PublicNode, 0, len(s.Children)), DocPath: s.DocPath,
		Projects: make([]PublicHitProject, 0, len(s.Projects))}
	for _, p := range s.Projects {
		out.Projects = append(out.Projects, PublicHitProject{Path: strings.Join(p.Path, "/"), Name: p.Name})
	}
	for _, a := range s.Ancestors {
		out.Ancestors = append(out.Ancestors, publicNode(a))
	}
	for _, c := range s.Children {
		n := publicNode(c.Node)
		n.HasDocs = new(c.HasDocs)
		out.Children = append(out.Children, n)
	}
	if d := s.Docs; d != nil {
		docs := PublicDocs{Branches: make([]PublicVersion, 0, len(d.Branches)), Tags: make([]PublicVersion, 0, len(d.Tags)), Release: d.Release}
		if d.Default != nil {
			docs.Default = new(publicVersion(*d.Default))
		}
		for _, b := range d.Branches {
			docs.Branches = append(docs.Branches, publicVersion(b))
		}
		for _, t := range d.Tags {
			docs.Tags = append(docs.Tags, publicVersion(t))
		}
		out.Docs = &docs
	}
	return out
}

type PublicSnapshot struct {
	Branch      string `json:"branch"`
	RefKind     string `json:"ref_kind"`
	Commit      string `json:"commit"`
	Status      string `json:"status"`
	CollectedAt string `json:"collected_at"`
}

func publicSnapshot(s knowledgetypes.Snapshot) PublicSnapshot {
	kind := string(s.RefKind)
	if kind == "" {
		kind = string(knowledgetypes.RefBranch)
	}
	return PublicSnapshot{Branch: s.Branch, RefKind: kind, Commit: s.Commit, Status: string(s.Status), CollectedAt: s.CollectedAt}
}

type PublicFile struct {
	Path       string  `json:"path"`
	Kind       string  `json:"kind"`
	Bytes      int64   `json:"bytes"`
	SkipReason *string `json:"skip_reason"`
	Title      *string `json:"title"`
}

func publicFile(f knowledgetypes.FileInfo) PublicFile {
	out := PublicFile{Path: f.Path, Kind: string(f.Kind), Bytes: f.Bytes, Title: ptr.NonEmpty(f.Title)}
	if f.Skip != nil {
		out.SkipReason = new(string(*f.Skip))
	}
	return out
}

type PublicDir struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Files int    `json:"files"`
}

type PublicTree struct {
	Snapshot PublicSnapshot `json:"snapshot"`
	Dirs     []PublicDir    `json:"dirs"`
	Files    []PublicFile   `json:"files"`
}

func PublicTreeOf(s knowledgetypes.Snapshot, l knowledgetypes.TreeLevel) PublicTree {
	out := PublicTree{Snapshot: publicSnapshot(s), Dirs: make([]PublicDir, 0, len(l.Dirs)), Files: make([]PublicFile, 0, len(l.Files))}
	for _, d := range l.Dirs {
		out.Dirs = append(out.Dirs, PublicDir{Name: d.Name, Path: d.Path, Files: d.Files})
	}
	for _, f := range l.Files {
		out.Files = append(out.Files, publicFile(f))
	}
	return out
}

type PublicFileContent struct {
	PublicFile
	Snapshot PublicSnapshot `json:"snapshot"`
	Content  *string        `json:"content"`
}

func PublicFileOf(s knowledgetypes.Snapshot, f knowledgetypes.FileContent) PublicFileContent {
	return PublicFileContent{PublicFile: publicFile(f.FileInfo), Snapshot: publicSnapshot(s), Content: f.Content}
}

type PublicHitProject struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type PublicHit struct {
	Project PublicHitProject         `json:"project"`
	Branch  string                   `json:"branch"`
	RefKind string                   `json:"ref_kind"`
	Path    string                   `json:"path"`
	Title   *string                  `json:"title"`
	Snippet []knowledgetypes.Segment `json:"snippet"`
}

type PublicSearch struct {
	Items      []PublicHit `json:"items"`
	NextCursor *string     `json:"next_cursor"`
}

func PublicSearchOf(hits []publishingservices.PublicHit, next string) PublicSearch {
	out := PublicSearch{Items: make([]PublicHit, 0, len(hits)), NextCursor: ptr.NonEmpty(next)}
	for _, h := range hits {
		snippet := h.Snippet
		if snippet == nil {
			snippet = []knowledgetypes.Segment{}
		}
		out.Items = append(out.Items, PublicHit{Project: PublicHitProject{Path: strings.Join(h.ProjectPath, "/"), Name: h.ProjectName},
			Branch: h.Branch, RefKind: string(h.RefKind), Path: h.Path, Title: ptr.NonEmpty(h.Title), Snippet: snippet})
	}
	return out
}

type PublicFiles struct {
	Snapshot PublicSnapshot `json:"snapshot"`
	Items    []PublicFile   `json:"items"`
}

func PublicFilesOf(s knowledgetypes.Snapshot, files []knowledgetypes.FileInfo) PublicFiles {
	out := PublicFiles{Snapshot: publicSnapshot(s), Items: make([]PublicFile, 0, len(files))}
	for _, f := range files {
		out.Items = append(out.Items, publicFile(f))
	}
	return out
}

type PublicAPIDocument struct {
	Snapshot PublicSnapshot `json:"snapshot"`
	apistypes.Document
}

func PublicAPIOf(s knowledgetypes.Snapshot, d apistypes.Document) PublicAPIDocument {
	return PublicAPIDocument{Snapshot: publicSnapshot(s), Document: d}
}
