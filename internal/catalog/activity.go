package catalog

import (
	"context"

	"github.com/google/uuid"
)

type ProcessKind string

const (
	ProcessCollect  ProcessKind = "collect"
	ProcessIndex    ProcessKind = "index"
	ProcessForge    ProcessKind = "forge"
	ProcessClusters ProcessKind = "clusters"
)

type ProcessState string

const (
	StateRunning ProcessState = "running"
	StateQueued  ProcessState = "queued"
	StateFailed  ProcessState = "failed"
	StateIdle    ProcessState = "idle"
)

type ProcessSignals struct {
	Kind    ProcessKind
	Running bool
	Queued  bool
	Failed  bool
	Code    *string
	LastAt  *string
	Pending *int64
}

type Process struct {
	Kind    ProcessKind
	State   ProcessState
	Code    *string
	LastAt  *string
	Pending *int64
}

type Summary struct {
	Running int64
	Queued  int64
	Failed  int64
}

type ActivityWant struct {
	Indexing bool
	Model    string
	Pending  bool
}

type Activity struct {
	Project   bool
	Processes []Process
	Summary   Summary
}

type ActivityStore interface {
	Signals(ctx context.Context, projects []uuid.UUID, want ActivityWant) (map[uuid.UUID][]ProcessSignals, error)
	Busy(ctx context.Context, want ActivityWant) (map[uuid.UUID][]ProcessSignals, error)
	Containers(ctx context.Context, userID uuid.UUID, all bool, projects, containers []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
}

func StateOf(s ProcessSignals) ProcessState {
	switch {
	case s.Running:
		return StateRunning
	case s.Queued:
		return StateQueued
	case s.Failed:
		return StateFailed
	}
	return StateIdle
}

func ProcessOf(s ProcessSignals) Process {
	p := Process{Kind: s.Kind, State: StateOf(s), LastAt: s.LastAt, Pending: s.Pending}
	if p.State == StateFailed {
		p.Code = s.Code
	}
	return p
}

func ProcessesOf(signals []ProcessSignals) []Process {
	out := make([]Process, 0, len(signals))
	for _, s := range signals {
		out = append(out, ProcessOf(s))
	}
	return out
}

func Has(processes []Process, state ProcessState) bool {
	for _, p := range processes {
		if p.State == state {
			return true
		}
	}
	return false
}

func Summarize(projects []uuid.UUID, processes map[uuid.UUID][]Process) Summary {
	var s Summary
	seen := map[uuid.UUID]bool{}
	for _, id := range projects {
		if seen[id] {
			continue
		}
		seen[id] = true
		p := processes[id]
		if Has(p, StateRunning) {
			s.Running++
		}
		if Has(p, StateQueued) {
			s.Queued++
		}
		if Has(p, StateFailed) {
			s.Failed++
		}
	}
	return s
}

func ParseProcessState(v string) (ProcessState, bool) {
	switch s := ProcessState(v); s {
	case StateRunning, StateQueued, StateFailed:
		return s, true
	}
	return "", false
}
