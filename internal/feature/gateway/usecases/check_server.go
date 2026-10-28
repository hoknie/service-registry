package usecases

import (
	"context"
	"time"

	"github.com/google/uuid"

	gatewayports "svc-registry/internal/feature/gateway/ports"
	"svc-registry/internal/feature/gateway/repositories"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
)

type Observer interface {
	CheckDone(outcome string)
	CallDone(server uuid.UUID, outcome string, seconds float64)
}

type CheckServerDeps struct {
	Checks   *repositories.CheckRepository
	Client   gatewayports.ClientPort
	Observer Observer
	Interval int
}

type CheckServerUseCase struct{ d CheckServerDeps }

func NewCheckServer(d CheckServerDeps) *CheckServerUseCase { return &CheckServerUseCase{d: d} }

type CheckInput struct {
	ServerID  uuid.UUID
	Target    gatewayports.Target
	TargetErr error
	Secrets   []string
}

func (u *CheckServerUseCase) Run(ctx context.Context, in CheckInput) (gatewaytypes.CheckRecord, error) {
	started := time.Now()
	var res *gatewaytypes.Result
	err := in.TargetErr
	if err == nil {
		res, err = u.d.Client.Check(ctx, in.Target)
	}
	rec := gatewaytypes.CheckOf(in.ServerID, res, err, int(time.Since(started).Milliseconds()), in.Secrets...)
	if u.d.Observer != nil {
		u.d.Observer.CheckDone(rec.Outcome)
	}
	return rec, u.d.Checks.Record(context.WithoutCancel(ctx), rec, started, u.d.Interval)
}
