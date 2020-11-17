// Package noop contains a compute backend that does nothing, for testing purposes.
package noop

import (
	"context"

	"github.com/ohsu-comp-bio/funnel/events"
)

// NewBackend returns a new noop Backend instance.
func NewBackend() *Backend {
	return &Backend{}
}

// Backend is a scheduler backend that doesn't do anything
// which is useful for testing.
type Backend struct{}

// WriteEvent is a noop and returns nil.
func (b *Backend) WriteEvent(context.Context, *events.Event) error {
	return nil
}

func (b *Backend) Close() {}
