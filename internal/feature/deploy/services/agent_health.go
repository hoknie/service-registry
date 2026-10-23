package services

import (
	"sync"
	"time"

	deploytypes "svc-registry/internal/feature/deploy/types"
)

const agentStallFloor = 10 * time.Minute

type AgentHealth struct {
	now      func() time.Time
	mu       sync.Mutex
	interval time.Duration
	beat     time.Time
	contact  time.Time
}

func NewAgentHealth(now func() time.Time) *AgentHealth {
	if now == nil {
		now = time.Now
	}
	return &AgentHealth{now: now, interval: deploytypes.DefaultInterval * time.Second, beat: now()}
}

func (h *AgentHealth) Beat(interval time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.remember(interval)
	h.beat = h.now()
}

func (h *AgentHealth) Contact(interval time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.remember(interval)
	h.beat = h.now()
	h.contact = h.beat
}

func (h *AgentHealth) Live() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.now().Sub(h.beat) <= max(3*h.interval, agentStallFloor)
}

func (h *AgentHealth) Ready() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return !h.contact.IsZero() && h.now().Sub(h.contact) <= 3*h.interval
}

func (h *AgentHealth) remember(interval time.Duration) {
	if interval > 0 {
		h.interval = interval
	}
}
