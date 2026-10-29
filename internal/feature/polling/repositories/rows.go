package repositories

import (
	"github.com/google/uuid"

	pollingtypes "svc-registry/internal/feature/polling/types"
)

type chainRow struct {
	ID           uuid.UUID `db:"id"`
	Kind         string    `db:"kind"`
	Name         string    `db:"name"`
	IntervalSecs *int      `db:"interval_secs"`
	TimeoutSecs  *int      `db:"timeout_secs"`
	Mode         *string   `db:"mode"`
}

func (r chainRow) chain() pollingtypes.ChainNode {
	own := pollingtypes.NodeSettings{IntervalSecs: r.IntervalSecs, TimeoutSecs: r.TimeoutSecs}
	if r.Mode != nil {
		own.Mode = new(pollingtypes.Mode(*r.Mode))
	}
	return pollingtypes.ChainNode{ID: r.ID, Kind: r.Kind, Name: r.Name, Own: own}
}

func modeParam(m *pollingtypes.Mode) *string {
	if m == nil {
		return nil
	}
	return new(string(*m))
}
