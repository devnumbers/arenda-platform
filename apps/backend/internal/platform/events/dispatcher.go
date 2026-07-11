package events

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// EventType identifies a class of domain events.
type EventType string

// Handler processes a single event.
type Handler func(ctx context.Context, event any) error

// Dispatcher routes events to subscribed handlers.
type Dispatcher interface {
	Subscribe(eventType EventType, handler Handler)
	Publish(ctx context.Context, eventType EventType, event any) error
}

// InProcessDispatcher is a simple synchronous in-process event dispatcher.
type InProcessDispatcher struct {
	mu       sync.RWMutex
	handlers map[EventType][]Handler
}

// NewInProcessDispatcher creates an empty dispatcher.
func NewInProcessDispatcher() *InProcessDispatcher {
	return &InProcessDispatcher{
		handlers: make(map[EventType][]Handler),
	}
}

// Subscribe registers a handler for the given event type.
func (d *InProcessDispatcher) Subscribe(eventType EventType, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventType] = append(d.handlers[eventType], handler)
}

// Publish invokes all handlers subscribed to the event type in order.
// If a handler returns an error, the remaining handlers are still invoked and
// the error is logged. An error joined from all failing handlers is returned.
func (d *InProcessDispatcher) Publish(ctx context.Context, eventType EventType, event any) error {
	d.mu.RLock()
	subscribers := d.handlers[eventType]
	handlers := make([]Handler, len(subscribers))
	copy(handlers, subscribers)
	d.mu.RUnlock()

	var errs []error
	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			slog.Default().ErrorContext(ctx, "event handler failed",
				slog.String("eventType", string(eventType)),
				slog.String("error", err.Error()))
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
