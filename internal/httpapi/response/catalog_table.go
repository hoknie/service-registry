package response

import (
	"svc-registry/internal/catalog"
	"svc-registry/internal/service"
)

type CatalogTableRow struct {
	Node
	Children int64       `json:"children"`
	Match    bool        `json:"match"`
	Activity any         `json:"activity"`
	Links    []LinkBadge `json:"links"`
}

type LinkBadge struct {
	LinkKey string   `json:"link_key"`
	KindKey string   `json:"kind_key"`
	Title   *string  `json:"title"`
	Icon    LinkIcon `json:"icon"`
	URL     *string  `json:"url"`
}

type CatalogTable struct {
	Items     []CatalogTableRow `json:"items"`
	Total     uint64            `json:"total"`
	Limit     uint32            `json:"limit"`
	Offset    uint64            `json:"offset"`
	Truncated bool              `json:"truncated"`
}

func TableNodes(page service.CatalogTablePage) []Node {
	out := make([]Node, 0, len(page.Items))
	for _, n := range page.Items {
		out = append(out, WalkedNode(n.WalkNode))
	}
	return out
}

func CatalogTableOf(page service.CatalogTablePage, nodes []Node) CatalogTable {
	rows := make([]CatalogTableRow, 0, len(page.Items))
	for i, n := range page.Items {
		var activity any
		switch {
		case n.Node.Kind == catalog.KindProject:
			activity = ProcessesOf(page.Processes[n.Node.ID])
		default:
			activity = SummaryOf(page.Summaries[n.Node.ID])
		}
		badges := []LinkBadge{}
		for _, b := range page.Links[n.Node.ID] {
			badges = append(badges, LinkBadge{LinkKey: b.LinkKey, KindKey: b.KindKey, Title: b.Title, Icon: LinkIconOf(b.Icon), URL: b.URL})
		}
		rows = append(rows, CatalogTableRow{Node: nodes[i], Children: n.Children, Match: n.Match, Activity: activity, Links: badges})
	}
	return CatalogTable{Items: rows, Total: page.Total, Limit: page.Limit, Offset: page.Offset, Truncated: page.Truncated}
}
