package leasejobs

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type queue struct {
	mu    sync.Mutex
	items []int
}

func (q *queue) Claim(_ context.Context, limit, _ int) ([]int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := min(limit, len(q.items))
	out := q.items[:n]
	q.items = q.items[n:]
	return out, nil
}

func (q *queue) Extend(context.Context, int, int) error { return nil }

func TestSchedulerRunsEveryClaimedItemWithinTheConcurrency(t *testing.T) {
	q := &queue{items: []int{1, 2, 3, 4, 5, 6}}
	var running, peak, done atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler[int]{Name: "test", Source: q, Concurrency: 2, Tick: 5 * time.Millisecond, Lease: time.Second,
		Run: func(context.Context, int) {
			n := running.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			running.Add(-1)
			done.Add(1)
		}}
	stopped := make(chan struct{})
	go func() { s.Serve(ctx); close(stopped) }()
	deadline := time.Now().Add(5 * time.Second)
	for done.Load() < 6 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-stopped
	if done.Load() != 6 {
		t.Fatalf("ran %d of 6", done.Load())
	}
	if peak.Load() > 2 {
		t.Fatalf("ran %d at once, concurrency 2", peak.Load())
	}
}

func TestAPanickingJobDoesNotStopTheScheduler(t *testing.T) {
	q := &queue{items: []int{1, 2}}
	var done atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler[int]{Name: "test", Source: q, Concurrency: 1, Tick: 5 * time.Millisecond, Lease: time.Second,
		Run: func(_ context.Context, item int) {
			done.Add(1)
			if item == 1 {
				panic("boom")
			}
		}}
	stopped := make(chan struct{})
	go func() { s.Serve(ctx); close(stopped) }()
	deadline := time.Now().Add(5 * time.Second)
	for done.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-stopped
	if done.Load() != 2 {
		t.Fatalf("ran %d of 2", done.Load())
	}
}
