package ports

import (
	"context"

	notifytypes "svc-registry/internal/feature/notify/types"
)

type SenderPort interface {
	Send(ctx context.Context, r notifytypes.Request) notifytypes.Response
}
