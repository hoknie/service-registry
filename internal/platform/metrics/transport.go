package metrics

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type roundTripper struct {
	m      *Metrics
	target string
	next   http.RoundTripper
}

func (m *Metrics) RoundTripper(target string, next http.RoundTripper) http.RoundTripper {
	if m == nil {
		return next
	}
	if next == nil {
		next = http.DefaultTransport
	}
	return roundTripper{m: m, target: target, next: next}
}

func (t roundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	res, err := t.next.RoundTrip(r)
	code := 0
	if res != nil {
		code = res.StatusCode
	}
	t.m.Outbound(t.target, ResultClass(code, err), time.Since(start))
	return res, err
}

func (m *Metrics) Client(target string, c *http.Client) *http.Client {
	if m == nil || c == nil {
		return c
	}
	wrapped := *c
	wrapped.Transport = m.RoundTripper(target, c.Transport)
	return &wrapped
}

func (m *Metrics) GRPCInterceptor(target string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		if m != nil {
			m.Outbound(target, grpcResult(err), time.Since(start))
		}
		return err
	}
}

func grpcResult(err error) string {
	if err == nil {
		return "2xx"
	}
	switch status.Code(err) {
	case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.PermissionDenied, codes.Unauthenticated,
		codes.FailedPrecondition, codes.OutOfRange, codes.ResourceExhausted:
		return "4xx"
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled, codes.Unknown:
		return "error"
	}
	return "5xx"
}
