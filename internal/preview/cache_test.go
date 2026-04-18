package preview

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir, 1024*1024)

	payload := []byte("fake PNG data")
	key := cache.Key("/videos/ep01.mkv", 21470*time.Millisecond)

	if cache.Exists(key) {
		t.Fatal("key should not exist yet")
	}

	if err := cache.Write(key, payload); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !cache.Exists(key) {
		t.Fatal("key should exist after write")
	}

	got, err := cache.Read(key)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("Read payload = %q, want %q", got, payload)
	}
}

func TestCacheKeyStability(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir, 1024*1024)

	k1 := cache.Key("/videos/ep01.mkv", 1000*time.Millisecond)
	k2 := cache.Key("/videos/ep01.mkv", 1000*time.Millisecond)
	if k1 != k2 {
		t.Errorf("same inputs produced different keys: %q vs %q", k1, k2)
	}

	k3 := cache.Key("/videos/ep01.mkv", 2000*time.Millisecond)
	if k1 == k3 {
		t.Errorf("different time should produce different keys")
	}

	k4 := cache.Key("/videos/ep02.mkv", 1000*time.Millisecond)
	if k1 == k4 {
		t.Errorf("different videoPath should produce different keys")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir, 30) // very small limit so evictions fire

	k1 := cache.Key("/a", 1*time.Second)
	k2 := cache.Key("/b", 1*time.Second)
	k3 := cache.Key("/c", 1*time.Second)

	data := make([]byte, 15)
	for i := range data {
		data[i] = 'x'
	}

	if err := cache.Write(k1, data); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := cache.Write(k2, data); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := cache.Write(k3, data); err != nil {
		t.Fatal(err)
	}

	// After k3 write, total = 45 > 30 limit → k1 (oldest) evicted
	if cache.Exists(k1) {
		t.Error("k1 should have been evicted (oldest)")
	}
	if !cache.Exists(k2) {
		t.Error("k2 should still exist")
	}
	if !cache.Exists(k3) {
		t.Error("k3 should still exist")
	}
}

func TestCacheCreatesDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "doesnotexist")
	cache := NewCache(dir, 1024)

	key := cache.Key("/v", time.Second)
	if err := cache.Write(key, []byte("x")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("cache dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("cache path is not a directory")
	}
}
