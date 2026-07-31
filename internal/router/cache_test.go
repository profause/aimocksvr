package router

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/models"
)

type fakeCache struct {
	store map[string][]byte
	gets  int
	sets  int
	miss  error
}

func newFakeCache() *fakeCache {
	return &fakeCache{store: map[string][]byte{}}
}

func (f *fakeCache) Get(_ context.Context, key string, dest any) (bool, error) {
	f.gets++
	if f.miss != nil {
		return false, f.miss
	}
	data, ok := f.store[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (f *fakeCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	f.sets++
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f.store[key] = data
	return nil
}

func (f *fakeCache) Delete(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(f.store, k)
	}
	return nil
}

func TestCachedEndpointStoreHitsCache(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{newEndpoint("GET", "/users")},
	}
	rc := newFakeCache()
	logger := zerolog.Nop()
	cached := NewCachedEndpointStore(store, rc, time.Minute, &logger)

	for i := 0; i < 2; i++ {
		got, err := cached.ListActiveByMethod(context.Background(), "GET")
		if err != nil {
			t.Fatalf("ListActiveByMethod failed: %v", err)
		}
		if len(got) != 1 || got[0].Path != "/users" {
			t.Fatalf("unexpected endpoints: %+v", got)
		}
	}

	if rc.gets != 2 {
		t.Errorf("expected 2 cache reads, got %d", rc.gets)
	}
	if rc.sets != 1 {
		t.Errorf("expected 1 cache write, got %d", rc.sets)
	}
	if store.calls != 1 {
		t.Errorf("expected 1 underlying store call, got %d", store.calls)
	}
}

func TestCachedEndpointStoreFallsBackOnCacheError(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{newEndpoint("GET", "/users")},
	}
	rc := newFakeCache()
	rc.miss = errors.New("redis down")
	logger := zerolog.Nop()
	cached := NewCachedEndpointStore(store, rc, time.Minute, &logger)

	got, err := cached.ListActiveByMethod(context.Background(), "GET")
	if err != nil {
		t.Fatalf("expected fallthrough on cache error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected endpoints from store, got %d", len(got))
	}
}

func TestCachedEndpointStorePassesThroughHistory(t *testing.T) {
	store := &fakeStore{}
	rc := newFakeCache()
	logger := zerolog.Nop()
	cached := NewCachedEndpointStore(store, rc, time.Minute, &logger)

	h := &models.RequestHistory{ID: uuid.New()}
	if err := cached.CreateHistory(context.Background(), h); err != nil {
		t.Fatalf("CreateHistory failed: %v", err)
	}
	if len(store.history) != 1 {
		t.Errorf("expected history passed through, got %d", len(store.history))
	}
}
