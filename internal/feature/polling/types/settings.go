package types

import (
	"time"

	"github.com/google/uuid"
)

type Mode string

const (
	ModeOn        Mode = "on"
	ModeOff       Mode = "off"
	ModePublished Mode = "published"
)

func (m Mode) Valid() bool { return m == ModeOn || m == ModeOff || m == ModePublished }

const (
	MinInterval = 60
	MaxInterval = 86400
	MinTimeout  = 1
	MaxTimeout  = 600
)

type NodeSettings struct {
	IntervalSecs *int
	TimeoutSecs  *int
	Mode         *Mode
}

func (n NodeSettings) IsEmpty() bool {
	return n.IntervalSecs == nil && n.TimeoutSecs == nil && n.Mode == nil
}

type ChainNode struct {
	ID   uuid.UUID
	Kind string
	Name string
	Own  NodeSettings
}

type Defaults struct {
	RepoSyncIntervalSecs int
	DocsIntervalSecs     int
	SkillsIntervalSecs   int
	TimeoutSecs          int
}

type From struct{ IntervalSecs, TimeoutSecs, Mode int }

type Effective struct {
	IntervalSecs *int
	TimeoutSecs  int
	Mode         Mode
}

func Merge(chain []ChainNode, timeoutSecs int) (Effective, From) {
	e, from := Effective{TimeoutSecs: timeoutSecs, Mode: ModeOn}, From{-1, -1, -1}
	for i := len(chain) - 1; i >= 0; i-- {
		own := chain[i].Own
		if own.IntervalSecs != nil {
			e.IntervalSecs, from.IntervalSecs = new(*own.IntervalSecs), i
		}
		if own.TimeoutSecs != nil {
			e.TimeoutSecs, from.TimeoutSecs = *own.TimeoutSecs, i
		}
		if own.Mode != nil {
			e.Mode, from.Mode = *own.Mode, i
		}
	}
	return e, from
}

func Validate(n NodeSettings) (NodeSettings, error) {
	if n.IntervalSecs != nil && (*n.IntervalSecs < MinInterval || *n.IntervalSecs > MaxInterval) {
		return NodeSettings{}, InvalidPolling
	}
	if n.TimeoutSecs != nil && (*n.TimeoutSecs < MinTimeout || *n.TimeoutSecs > MaxTimeout) {
		return NodeSettings{}, InvalidPolling
	}
	if n.Mode != nil && !n.Mode.Valid() {
		return NodeSettings{}, InvalidPolling
	}
	return n, nil
}

func (e Effective) IntervalOr(serverSecs int) int {
	if e.IntervalSecs != nil {
		return *e.IntervalSecs
	}
	return serverSecs
}

func (e Effective) Timeout() time.Duration { return time.Duration(e.TimeoutSecs) * time.Second }

func (e Effective) Off() bool { return e.Mode == ModeOff }

func (e Effective) DocsPaused(published bool) bool {
	return e.Mode == ModeOff || (e.Mode == ModePublished && !published)
}
