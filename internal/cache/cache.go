// Package cache provides a key/value cache used across the application.
//
// Redis is optional: when it is not configured, a Noop cache is returned so
// callers never need nil checks. Failures degrade gracefully to uncached
// behavior instead of taking the server down.
package cache

import (
	"context"
	"time"
)

// Cache is a key/value store for computed data. Values are JSON-encoded.
type Cache interface {
	// Get reads a key into dest. The bool result reports whether the key was
	// present; a miss is not an error.
	Get(ctx context.Context, key string, dest any) (bool, error)
	// Set stores a value with a time-to-live.
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	// Delete removes one or more keys.
	Delete(ctx context.Context, keys ...string) error
}

// Noop is a cache that stores nothing. Used when Redis is not configured.
type Noop struct{}

func (Noop) Get(context.Context, string, any) (bool, error)        { return false, nil }
func (Noop) Set(context.Context, string, any, time.Duration) error { return nil }
func (Noop) Delete(context.Context, ...string) error               { return nil }
