package repositories

import (
	pollingtypes "svc-registry/internal/feature/polling/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

var constraints = postgres.Constraints{
	"node_polling_node_id_fkey":        apperr.ErrNotFound,
	"node_polling_interval_secs_check": pollingtypes.InvalidPolling,
	"node_polling_timeout_secs_check":  pollingtypes.InvalidPolling,
	"node_polling_mode_check":          pollingtypes.InvalidPolling,
}

func Err(err error) error { return postgres.Translate(err, constraints) }
