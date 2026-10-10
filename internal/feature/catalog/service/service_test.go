package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/config"
)

type stubPorts struct{}

func (stubPorts) Managed(context.Context, []uuid.UUID) (map[uuid.UUID]bool, error) { return nil, nil }

func (stubPorts) ActivitySignals(context.Context, []uuid.UUID, bool, bool) (map[uuid.UUID][]catalog.ProcessSignals, error) {
	return nil, nil
}

func TestServiceIsWiredOnlyAfterUse(t *testing.T) {
	t.Parallel()
	s := New(Deps{Config: &config.Config{}})
	if s.Wired() {
		t.Fatal("a new service must not be wired")
	}
	s.Use(stubPorts{}, stubPorts{})
	if !s.Wired() {
		t.Fatal("a service with ports must be wired")
	}
}

func TestSignalsMergeSourcesInOrder(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	s := New(Deps{Config: &config.Config{}})
	s.Use(stubPorts{}, fixedSignals{id: id, kind: catalog.ProcessKind("collect")}, fixedSignals{id: id, kind: catalog.ProcessKind("forge")})
	got, err := s.signals(context.Background(), []uuid.UUID{id}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got[id]) != 2 || got[id][0].Kind != "collect" || got[id][1].Kind != "forge" {
		t.Fatalf("signals = %+v, want collect then forge", got[id])
	}
	empty, err := s.signals(context.Background(), nil, false, false)
	if err != nil || len(empty) != 0 {
		t.Fatalf("no projects: %v %v, want an empty map", empty, err)
	}
}

type fixedSignals struct {
	id   uuid.UUID
	kind catalog.ProcessKind
}

func (f fixedSignals) ActivitySignals(context.Context, []uuid.UUID, bool, bool) (map[uuid.UUID][]catalog.ProcessSignals, error) {
	return map[uuid.UUID][]catalog.ProcessSignals{f.id: {{Kind: f.kind}}}, nil
}
