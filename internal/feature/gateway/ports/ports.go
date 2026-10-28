package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	gatewaytypes "svc-registry/internal/feature/gateway/types"
)

type Target struct {
	ServerID   uuid.UUID
	Version    string
	URL        string
	Transport  string
	AuthHeader string
	AuthValue  string
	Proxy      gatewaytypes.Proxy
	ProxyAuth  string
	Timeout    time.Duration
	MaxBytes   int64
}

type CallResult struct {
	Raw     json.RawMessage
	IsError bool
}

type ClientPort interface {
	Check(ctx context.Context, t Target) (*gatewaytypes.Result, error)
	Call(ctx context.Context, t Target, tool string, args json.RawMessage) (*CallResult, error)
}
