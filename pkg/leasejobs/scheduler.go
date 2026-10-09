package leasejobs

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Source[T any] interface {
	Claim(ctx context.Context, limit, leaseSecs int) ([]T, error)
	Extend(ctx context.Context, item T, leaseSecs int) error
}

type Scheduler[T any] struct {
	Name        string
	Source      Source[T]
	Run         func(ctx context.Context, item T)
	Concurrency int
	Tick        time.Duration
	Lease       time.Duration
}

func (s *Scheduler[T]) Serve(ctx context.Context) {
	tick, lease := s.Tick, s.Lease
	if tick <= 0 {
		tick = 5 * time.Second
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	sem := make(chan struct{}, max(s.Concurrency, 1))
	var wg sync.WaitGroup
	defer wg.Wait()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		if free := cap(sem) - len(sem); free > 0 {
			items, err := s.Source.Claim(ctx, free, int(lease/time.Second))
			if err != nil && ctx.Err() == nil {
				slog.Warn("background job claim failed", "job", s.Name, "error", err)
			}
			for _, item := range items {
				sem <- struct{}{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					s.runOne(ctx, item, lease)
				}()
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Scheduler[T]) runOne(ctx context.Context, item T, lease time.Duration) {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(lease / 3)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if err := s.Source.Extend(context.WithoutCancel(ctx), item, int(lease/time.Second)); err != nil {
					slog.Warn("background job lease not extended", "job", s.Name, "error", err)
				}
			}
		}
	}()
	defer close(done)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("background job panicked", "job", s.Name, "panic", r)
		}
	}()
	s.Run(ctx, item)
}
