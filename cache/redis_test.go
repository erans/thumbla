package cache

import (
	"bytes"
	"encoding/gob"
	"image"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/erans/thumbla/config"
	"github.com/redis/go-redis/v9/logging"
)

func TestMain(m *testing.M) {
	logging.Disable()
	os.Exit(m.Run())
}

func TestRedisCache_SetGetBytes_RoundTrip(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	cfg := &config.Config{}
	cfg.Cache.Redis.Address = mr.Addr()
	cfg.Cache.Redis.TTL = 60
	cfg.Cache.Redis.MaxLRUSize = 100
	config.SetConfig(cfg)

	c := NewRedisCache(cfg)

	rects := []image.Rectangle{image.Rect(0, 0, 10, 10), image.Rect(5, 5, 20, 20)}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(rects); err != nil {
		t.Fatal(err)
	}
	c.SetBytes("face-key", buf.Bytes())

	got, ok := c.GetBytes("face-key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	var out []image.Rectangle
	if err := gob.NewDecoder(bytes.NewReader(got)).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 2 || out[0] != rects[0] || out[1] != rects[1] {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
}

func TestRedisCache_TTLApplied(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	cfg := &config.Config{}
	cfg.Cache.Redis.Address = mr.Addr()
	cfg.Cache.Redis.TTL = 60
	config.SetConfig(cfg)

	c := NewRedisCache(cfg)
	c.SetBytes("k", []byte("v"))
	ttl := mr.TTL("k")
	if ttl <= 0 {
		t.Fatal("expected a positive TTL on the key")
	}
}

func TestRedisCache_TrimsDeleteDataKeys(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	cfg := &config.Config{}
	cfg.Cache.Redis.Address = mr.Addr()
	cfg.Cache.Redis.TTL = 60
	cfg.Cache.Redis.MaxLRUSize = 1 // tiny to force eviction
	config.SetConfig(cfg)

	c := NewRedisCache(cfg)
	c.SetBytes("k1", []byte("v1"))
	c.SetBytes("k2", []byte("v2")) // should evict one data key

	remaining := 0
	if c.Contains("k1") {
		remaining++
	}
	if c.Contains("k2") {
		remaining++
	}
	if remaining != 1 {
		t.Fatalf("expected exactly one data key after LRU trim, got %d", remaining)
	}
}
