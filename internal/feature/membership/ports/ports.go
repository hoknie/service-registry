package ports

import (
	"context"

	membershiptypes "svc-registry/internal/feature/membership/types"
)

type MailerPort interface {
	Enabled() bool
	Send(ctx context.Context, letter membershiptypes.Letter) error
}
