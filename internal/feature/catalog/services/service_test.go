package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/config"
)

type stubPorts struct{}

func (stubPorts) Managed(context.Context, []uuid.UUID) (map[uuid.UUID]bool, error) { return nil, nil }

func (stubPorts) SecretUsage(context.Context, []uuid.UUID) (map[uuid.UUID]int64, error) {
	return nil, nil
}

func (stubPorts) ActivitySignals(context.Context, []uuid.UUID, bool, bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error) {
	return nil, nil
}

func TestServiceIsWiredOnlyAfterUse(t *testing.T) {
	t.Parallel()
	s := New(Deps{Config: &config.Config{}})
	if s.Wired() {
		t.Fatal("a new service must not be wired")
	}
	s.Use(stubPorts{}, stubPorts{})
	if s.Wired() {
		t.Fatal("a service without secret users must not be wired")
	}
	s.UseSecretUsers(stubPorts{})
	if !s.Wired() {
		t.Fatal("a service with ports must be wired")
	}
}

func TestSignalsMergeSourcesInOrder(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	s := New(Deps{Config: &config.Config{}})
	s.Use(stubPorts{}, fixedSignals{id: id, kind: catalogtypes.ProcessKind("collect")}, fixedSignals{id: id, kind: catalogtypes.ProcessKind("forge")})
	got, err := s.Activity.signals(context.Background(), []uuid.UUID{id}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got[id]) != 2 || got[id][0].Kind != "collect" || got[id][1].Kind != "forge" {
		t.Fatalf("signals = %+v, want collect then forge", got[id])
	}
	empty, err := s.Activity.signals(context.Background(), nil, false, false)
	if err != nil || len(empty) != 0 {
		t.Fatalf("no projects: %v %v, want an empty map", empty, err)
	}
}

type fixedSignals struct {
	id   uuid.UUID
	kind catalogtypes.ProcessKind
}

func (f fixedSignals) ActivitySignals(context.Context, []uuid.UUID, bool, bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error) {
	return map[uuid.UUID][]catalogtypes.ProcessSignals{f.id: {{Kind: f.kind}}}, nil
}

type managedPort struct {
	managed map[uuid.UUID]bool
	asked   *[]uuid.UUID
	err     error
}

func (m managedPort) Managed(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	*m.asked = append(*m.asked, ids...)
	return m.managed, m.err
}

func TestMarkWalkedAsksOnlyReadableNodes(t *testing.T) {
	t.Parallel()
	readable, stub := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	var asked []uuid.UUID
	s := New(Deps{Config: &config.Config{}})
	s.Use(managedPort{managed: map[uuid.UUID]bool{readable: true, stub: true}, asked: &asked}, stubPorts{})
	nodes := []catalogtypes.WalkNode{{Node: catalogtypes.Node{ID: readable}, Readable: true}, {Node: catalogtypes.Node{ID: stub}}}
	if err := s.Nodes.markWalked(context.Background(), nodes); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != readable {
		t.Fatalf("asked = %v, want only the readable node", asked)
	}
	if !nodes[0].Node.Managed {
		t.Fatal("the readable node must be marked managed")
	}
}

func TestMarkWalkedSkipsThePortWithoutReadableNodes(t *testing.T) {
	t.Parallel()
	var asked []uuid.UUID
	s := New(Deps{Config: &config.Config{}})
	s.Use(managedPort{asked: &asked, err: errors.New("unreachable")}, stubPorts{})
	if err := s.Nodes.markWalked(context.Background(), []catalogtypes.WalkNode{{Node: catalogtypes.Node{ID: uuid.Must(uuid.NewV7())}}}); err != nil {
		t.Fatalf("no readable nodes: %v, want no call", err)
	}
	if len(asked) != 0 {
		t.Fatalf("asked = %v, want no call", asked)
	}
}

func TestMarkReturnsThePortError(t *testing.T) {
	t.Parallel()
	var asked []uuid.UUID
	s := New(Deps{Config: &config.Config{}})
	s.Use(managedPort{asked: &asked, err: errors.New("forge is down")}, stubPorts{})
	if err := s.Nodes.mark(context.Background(), new(catalogtypes.Node{ID: uuid.Must(uuid.NewV7())})); err == nil {
		t.Fatal("the port error must reach the caller")
	}
}
