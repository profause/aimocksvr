package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/config"
)

type payload struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type testRedis struct {
	server *miniredis.Miniredis
	cache  Cache
}

func newTestRedis(t *testing.T) *testRedis {
	t.Helper()
	s := miniredis.RunT(t)
	cfg := &config.Config{}
	cfg.Cache.Redis.Addr = s.Addr()
	cfg.Cache.Redis.DB = 0
	tr := &testRedis{server: s}
	tr.cache = &redisCache{client: redis.NewClient(&redis.Options{Addr: s.Addr()})}
	t.Cleanup(func() { _ = tr.cache.(*redisCache).client.Close() })
	return tr
}

func TestRedisCacheRoundTrip(t *testing.T) {
	tr := newTestRedis(t)

	want := payload{Name: "ada", Age: 37}
	if err := tr.cache.Set(context.Background(), "user:1", want, time.Minute); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	var got payload
	hit, err := tr.cache.Get(context.Background(), "user:1", &got)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !hit {
		t.Fatal("expected a cache hit")
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRedisCacheMiss(t *testing.T) {
	tr := newTestRedis(t)

	var got payload
	hit, err := tr.cache.Get(context.Background(), "missing", &got)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if hit {
		t.Fatal("expected a cache miss")
	}
}

func TestRedisCacheDelete(t *testing.T) {
	tr := newTestRedis(t)

	if err := tr.cache.Set(context.Background(), "a", "1", time.Minute); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if err := tr.cache.Set(context.Background(), "b", "2", time.Minute); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if err := tr.cache.Delete(context.Background(), "a", "b"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	for _, key := range []string{"a", "b"} {
		hit, _ := tr.cache.Get(context.Background(), key, &payload{})
		if hit {
			t.Errorf("expected %q to be deleted", key)
		}
	}
}

func TestRedisCacheTTLExpires(t *testing.T) {
	tr := newTestRedis(t)

	if err := tr.cache.Set(context.Background(), "temp", "1", time.Millisecond); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	tr.server.FastForward(time.Second)

	var got payload
	hit, err := tr.cache.Get(context.Background(), "temp", &got)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if hit {
		t.Fatal("expected expired key to miss")
	}
}

func TestNewReturnsNoopWhenRedisNotConfigured(t *testing.T) {
	logger := zerolog.Nop()
	cfg := &config.Config{}
	if _, ok := New(cfg, &logger).(Noop); !ok {
		t.Fatal("expected Noop cache when redis addr is empty")
	}
}
