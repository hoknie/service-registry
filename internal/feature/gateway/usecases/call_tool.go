package usecases

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	gatewayports "svc-registry/internal/feature/gateway/ports"
	"svc-registry/internal/feature/gateway/repositories"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
)

type CallToolDeps struct {
	Calls    *repositories.CallRepository
	Checks   *repositories.CheckRepository
	Client   gatewayports.ClientPort
	Observer Observer
}

type CallToolUseCase struct{ d CallToolDeps }

func NewCallTool(d CallToolDeps) *CallToolUseCase { return &CallToolUseCase{d: d} }

type CallInput struct {
	ServerID  uuid.UUID
	Tool      string
	Args      json.RawMessage
	Caller    gatewaytypes.Caller
	Target    gatewayports.Target
	TargetErr error
	Secrets   []string
	Log       bool
}

func (u *CallToolUseCase) Run(ctx context.Context, in CallInput) (*gatewayports.CallResult, error) {
	started := time.Now()
	var res *gatewayports.CallResult
	err := in.TargetErr
	if err == nil {
		res, err = u.d.Client.Call(ctx, in.Target, in.Tool, in.Args)
	}
	id, idErr := uuid.NewV7()
	if idErr != nil {
		return nil, idErr
	}
	rec := gatewaytypes.CallRecord{ID: id, ServerID: in.ServerID, Tool: in.Tool, Caller: in.Caller,
		DurationMs: int(time.Since(started).Milliseconds()), Outcome: gatewaytypes.CallOK}
	switch {
	case err != nil:
		code := gatewaytypes.FailureCode(err)
		rec.Outcome, rec.Code = gatewaytypes.CallFailed, &code
		err = gatewaytypes.Fail(code, gatewaytypes.Redact(gatewaytypes.FailureDetail(err), in.Secrets...))
	case res.IsError:
		rec.Outcome = gatewaytypes.CallToolError
	}
	if in.Log {
		rec.Request, rec.Response, rec.Truncated = payloads(in, res, err)
	}
	quiet := context.WithoutCancel(ctx)
	if logErr := u.d.Calls.Record(quiet, rec); logErr != nil {
		slog.Warn("mcp gateway: the call was not logged", "server", in.ServerID, "error", logErr)
	}
	if rec.Code != nil && *rec.Code == gatewaytypes.FailProtocol {
		_ = u.d.Checks.Requeue(quiet, in.ServerID)
	}
	if u.d.Observer != nil {
		u.d.Observer.CallDone(in.ServerID, rec.Outcome, time.Since(started).Seconds())
	}
	return res, err
}

func payloads(in CallInput, res *gatewayports.CallResult, err error) (*string, *string, bool) {
	clip := func(text string) (*string, bool) {
		out, cut := gatewaytypes.Clip(gatewaytypes.Redact(text, in.Secrets...))
		return &out, cut
	}
	request, cutRequest := clip(string(in.Args))
	var answer string
	switch {
	case err != nil:
		answer = err.Error()
	case res != nil:
		answer = string(res.Raw)
	}
	response, cutResponse := clip(answer)
	return request, response, cutRequest || cutResponse
}
