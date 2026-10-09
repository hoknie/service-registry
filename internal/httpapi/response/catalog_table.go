package response

import (
	"svc-registry/internal/catalog"
	"svc-registry/internal/service"
)

type CatalogTableRow struct {
	Node
	Children int64 `json:"children"`
	Match    bool  `json:"match"`
	Activity any   `json:"activity"`
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
		rows = append(rows, CatalogTableRow{Node: nodes[i], Children: n.Children, Match: n.Match, Activity: activity})
	}
	return CatalogTable{Items: rows, Total: page.Total, Limit: page.Limit, Offset: page.Offset, Truncated: page.Truncated}
}
