package core

import (
	"context"
	"sync"
)

type downloadLimiter struct {
	mu     sync.Mutex
	limit  int
	inUse  int
	notify chan struct{}
}

func newDownloadLimiter(limit int) *downloadLimiter {
	if limit < 1 {
		limit = 1
	}
	return &downloadLimiter{limit: limit, notify: make(chan struct{})}
}

func (l *downloadLimiter) Acquire(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		l.mu.Lock()
		if l.inUse < l.limit {
			l.inUse++
			l.mu.Unlock()
			return nil
		}
		notify := l.notify
		l.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-notify:
		}
	}
}

func (l *downloadLimiter) Release() {
	l.mu.Lock()
	if l.inUse > 0 {
		l.inUse--
	}
	l.signalLocked()
	l.mu.Unlock()
}

func (l *downloadLimiter) SetLimit(limit int) {
	if limit < 1 {
		limit = 1
	}
	l.mu.Lock()
	l.limit = limit
	l.signalLocked()
	l.mu.Unlock()
}

func (l *downloadLimiter) signalLocked() {
	close(l.notify)
	l.notify = make(chan struct{})
}
