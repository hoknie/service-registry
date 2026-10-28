package mcpfake

import (
	"context"
	"net/http"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearer struct{ value string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", b.value)
	return http.DefaultTransport.RoundTrip(r)
}

func TestFakeServerAnswersWithItsTools(t *testing.T) {
	t.Parallel()
	s := Start(t, Tool{Name: "search", Description: "Search issues"})
	s.RequireHeader("Authorization", "Bearer secret")
	ctx := context.Background()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	transport := &sdk.StreamableClientTransport{Endpoint: s.URL, HTTPClient: &http.Client{Transport: bearer{"Bearer secret"}}, MaxRetries: -1}
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "search" {
		t.Fatalf("tools %+v %v", tools, err)
	}
	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "search", Arguments: map[string]any{"q": "bug"}})
	if err != nil || res.IsError {
		t.Fatalf("call %+v %v", res, err)
	}
	if calls := s.Calls(); len(calls) != 1 || calls[0].Auth != "Bearer secret" || string(calls[0].Args) != `{"q":"bug"}` {
		t.Fatalf("calls %+v", calls)
	}
}

func TestFakeServerRefusesAWrongHeader(t *testing.T) {
	t.Parallel()
	s := Start(t)
	s.RequireHeader("X-Api-Key", "k")
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	if _, err := client.Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: s.URL, MaxRetries: -1}, nil); err == nil {
		t.Fatal("connected without the header")
	}
}
