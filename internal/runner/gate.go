package runner

import (
	"context"
	"sync"
)

type concurrencyGate struct {
	mu    sync.Mutex
	cond  *sync.Cond
	limit int
	inUse int
}

func newConcurrencyGate(limit int) *concurrencyGate {
	if limit < 1 {
		limit = 1
	}
	g := &concurrencyGate{limit: limit}
	g.cond = sync.NewCond(&g.mu)
	return g
}

func (g *concurrencyGate) setLimit(limit int) {
	if limit < 1 {
		limit = 1
	}
	g.mu.Lock()
	g.limit = limit
	g.cond.Broadcast()
	g.mu.Unlock()
}

func (g *concurrencyGate) acquire(ctx context.Context) error {
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			g.mu.Lock()
			g.cond.Broadcast()
			g.mu.Unlock()
		case <-done:
		}
	}()

	g.mu.Lock()
	defer g.mu.Unlock()
	for g.inUse >= g.limit {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		g.cond.Wait()
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	g.inUse++
	return nil
}

func (g *concurrencyGate) release() {
	g.mu.Lock()
	if g.inUse > 0 {
		g.inUse--
	}
	g.cond.Broadcast()
	g.mu.Unlock()
}
