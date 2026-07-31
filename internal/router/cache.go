package router

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/cache"
	"github.com/profause/aimocksvr/internal/models"
)

// cachedEndpointStore is a read-through cache around an EndpointStore. The
// active endpoint list per method is cached to avoid a database query on every
// mock request; request history still passes straight through.
type cachedEndpointStore struct {
	next   EndpointStore
	cache  cache.Cache
	ttl    time.Duration
	logger *zerolog.Logger
}

// NewCachedEndpointStore wraps a store with a read-through cache.
func NewCachedEndpointStore(next EndpointStore, c cache.Cache, ttl time.Duration, logger *zerolog.Logger) EndpointStore {
	return &cachedEndpointStore{next: next, cache: c, ttl: ttl, logger: logger}
}

func (s *cachedEndpointStore) ListActiveByMethod(ctx context.Context, method string) ([]models.Endpoint, error) {
	key := cache.EndpointListKey(method)

	var endpoints []models.Endpoint
	hit, err := s.cache.Get(ctx, key, &endpoints)
	if err == nil && hit {
		return endpoints, nil
	}
	if err != nil {
		s.logger.Warn().Err(err).Str("method", method).Msg("endpoint cache read failed")
	}

	endpoints, err = s.next.ListActiveByMethod(ctx, method)
	if err != nil {
		return nil, err
	}

	if err := s.cache.Set(ctx, key, endpoints, s.ttl); err != nil {
		s.logger.Warn().Err(err).Str("method", method).Msg("endpoint cache write failed")
	}
	return endpoints, nil
}

func (s *cachedEndpointStore) CreateHistory(ctx context.Context, h *models.RequestHistory) error {
	return s.next.CreateHistory(ctx, h)
}
