package types

import (
	"strings"

	"github.com/google/uuid"
)

type EntryKind string

const (
	EntryPath EntryKind = "path"
	EntryHost EntryKind = "host"
)

type Entry struct {
	Kind    EntryKind
	Segment string
	Host    string
}

func entriesOf(n ChainNode) []Entry {
	var out []Entry
	switch {
	case n.Own.Name != nil:
		out = append(out, Entry{Kind: EntryPath, Segment: *n.Own.Name})
	case n.Kind == "organization":
		out = append(out, Entry{Kind: EntryPath, Segment: n.Slug})
	}
	if n.Own.Host != nil {
		out = append(out, Entry{Kind: EntryHost, Host: *n.Own.Host})
	}
	return out
}

func IsEntry(n ChainNode) bool { return len(entriesOf(n)) > 0 }

func Entries(chain []ChainNode) []Entry {
	published := make([]bool, len(chain))
	s := DefaultSettings()
	for i := len(chain) - 1; i >= 0; i-- {
		s = s.Apply(chain[i].Own)
		published[i] = s.Publish
	}
	var out []Entry
	for i := 0; i < len(chain) && published[i]; i++ {
		out = append(out, entriesOf(chain[i])...)
	}
	return out
}

type Node struct {
	ID          uuid.UUID
	ParentID    *uuid.UUID
	Kind        string
	Slug        string
	Name        string
	Description string
	Own         NodeSettings
}

type Public struct {
	Entry    Node
	settings map[uuid.UUID]Settings
	nodes    map[uuid.UUID]Node
	children map[uuid.UUID][]uuid.UUID
}

func BuildPublic(entryChain []ChainNode, entry Node, subtree []Node) (*Public, bool) {
	root, _ := Merge(entryChain)
	if !root.Publish {
		return nil, false
	}
	p := &Public{Entry: entry, settings: map[uuid.UUID]Settings{entry.ID: root}, nodes: map[uuid.UUID]Node{entry.ID: entry},
		children: map[uuid.UUID][]uuid.UUID{}}
	for _, n := range subtree {
		if n.ParentID == nil {
			continue
		}
		parent, ok := p.settings[*n.ParentID]
		if !ok {
			continue
		}
		s := parent.Apply(n.Own)
		if !s.Publish {
			continue
		}
		p.settings[n.ID], p.nodes[n.ID] = s, n
		p.children[*n.ParentID] = append(p.children[*n.ParentID], n.ID)
	}
	return p, true
}

func (p *Public) Settings(id uuid.UUID) Settings { return p.settings[id] }

func (p *Public) Children(id uuid.UUID) []Node {
	out := make([]Node, 0, len(p.children[id]))
	for _, c := range p.children[id] {
		out = append(out, p.nodes[c])
	}
	return out
}

func (p *Public) Walk(segments []string) (Node, []Node, string, bool) {
	cur := p.Entry
	var ancestors []Node
	for i, seg := range segments {
		if cur.Kind == "project" {
			return cur, ancestors, strings.Join(segments[i:], "/"), true
		}
		next, found := Node{}, false
		for _, c := range p.Children(cur.ID) {
			if strings.EqualFold(c.Slug, seg) {
				next, found = c, true
				break
			}
		}
		if !found {
			return Node{}, nil, "", false
		}
		ancestors = append(ancestors, cur)
		cur = next
	}
	return cur, ancestors, "", true
}

func (p *Public) Projects(under uuid.UUID) []Node {
	var out []Node
	var walk func(id uuid.UUID)
	walk = func(id uuid.UUID) {
		n := p.nodes[id]
		if n.Kind == "project" {
			out = append(out, n)
			return
		}
		for _, c := range p.children[id] {
			walk(c)
		}
	}
	walk(under)
	return out
}

func (p *Public) PathOf(id uuid.UUID) []string {
	var out []string
	for cur := p.nodes[id]; cur.ID != p.Entry.ID; cur = p.nodes[*cur.ParentID] {
		out = append([]string{cur.Slug}, out...)
	}
	return out
}
