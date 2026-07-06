package events

import (
	"context"
	"sync"

	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
)

// InProcessDispatcher is a simple synchronous event dispatcher.
type InProcessDispatcher struct {
	mu                     sync.RWMutex
	userRegisteredHandlers []func(ctx context.Context, event identityapp.UserRegistered) error
}

func NewInProcessDispatcher() *InProcessDispatcher {
	return &InProcessDispatcher{}
}

func (d *InProcessDispatcher) OnUserRegistered(h func(ctx context.Context, event identityapp.UserRegistered) error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.userRegisteredHandlers = append(d.userRegisteredHandlers, h)
}

func (d *InProcessDispatcher) PublishUserRegistered(ctx context.Context, event identityapp.UserRegistered) error {
	d.mu.RLock()
	handlers := make([]func(ctx context.Context, event identityapp.UserRegistered) error, len(d.userRegisteredHandlers))
	copy(handlers, d.userRegisteredHandlers)
	d.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
