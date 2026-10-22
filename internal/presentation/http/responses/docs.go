package responses

import (
	"github.com/google/uuid"

	knowledgeservices "svc-registry/internal/feature/knowledge/services"
)

type DocSpaceNode struct {
	ID          uuid.UUID `json:"id"`
	Kind        string    `json:"kind"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Access      string    `json:"access"`
	HasDocs     *bool     `json:"has_docs,omitempty"`
}

type DocSpaceBranch struct {
	Name        string `json:"name"`
	CollectedAt string `json:"collected_at"`
}

type DocSpaceDocs struct {
	Source        *string          `json:"source"`
	DefaultBranch string           `json:"default_branch"`
	Branches      []DocSpaceBranch `json:"branches"`
}

type DocSpace struct {
	Node      *DocSpaceNode  `json:"node"`
	Ancestors []DocSpaceNode `json:"ancestors"`
	Children  []DocSpaceNode `json:"children"`
	Docs      *DocSpaceDocs  `json:"docs"`
	DocPath   *string        `json:"doc_path"`
}

func docSpaceNode(n knowledgeservices.DocSpaceNode, withDocs bool) DocSpaceNode {
	out := DocSpaceNode{ID: n.ID, Kind: string(n.Kind), Slug: n.Slug, Name: n.Name, Description: n.Description, Access: "navigate"}
	if n.Readable {
		out.Access = "read"
	}
	if withDocs {
		out.HasDocs = new(n.HasDocs)
	}
	return out
}

func DocSpaceOf(s knowledgeservices.DocSpace) DocSpace {
	out := DocSpace{Ancestors: make([]DocSpaceNode, 0, len(s.Ancestors)), Children: make([]DocSpaceNode, 0, len(s.Children)), DocPath: s.DocPath}
	if s.Node != nil {
		n := docSpaceNode(*s.Node, false)
		if n.Description == nil {
			n.Description = new("")
		}
		out.Node = &n
	}
	for _, a := range s.Ancestors {
		out.Ancestors = append(out.Ancestors, docSpaceNode(a, false))
	}
	for _, c := range s.Children {
		out.Children = append(out.Children, docSpaceNode(c, true))
	}
	if d := s.Docs; d != nil {
		docs := DocSpaceDocs{DefaultBranch: d.DefaultBranch, Branches: make([]DocSpaceBranch, 0, len(d.Branches))}
		if d.Source != "" {
			docs.Source = new(string(d.Source))
		}
		for _, b := range d.Branches {
			docs.Branches = append(docs.Branches, DocSpaceBranch(b))
		}
		out.Docs = &docs
	}
	return out
}
