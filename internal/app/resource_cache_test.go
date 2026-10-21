package app

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestResourceNameCacheLoadsOnce(t *testing.T) {
	var calls atomic.Int64
	cache := newResourceNameCache(func(id int) cachedResourceName {
		calls.Add(1)
		if id == 42 {
			return cachedResourceName{name: "火球", found: true}
		}
		return cachedResourceName{name: "0", found: false}
	})

	if name, found := cache.lookup(42); name != "火球" || !found {
		t.Fatalf("lookup(42) = %q, %v; want 火球, true", name, found)
	}
	for i := 0; i < 100; i++ {
		if name, found := cache.lookup(42); name != "火球" || !found {
			t.Fatalf("cached lookup(42) = %q, %v", name, found)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("load called %d times, want 1", got)
	}

	// 未命中的 ID 也必须只回源一次（负缓存），否则未知 ID 会在热路径上反复查库。
	for i := 0; i < 50; i++ {
		if name, found := cache.lookup(7); found || name != "0" {
			t.Fatalf("miss lookup(7) = %q, %v; want 0, false", name, found)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("load called %d times, want 2 (misses must be cached too)", got)
	}

	if name := cache.name(42); name != "火球" {
		t.Fatalf("name(42) = %q", name)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("name() must hit the cache, load called %d times", got)
	}
}

func TestResourceNameCacheConcurrentLookups(t *testing.T) {
	var calls atomic.Int64
	cache := newResourceNameCache(func(id int) cachedResourceName {
		calls.Add(1)
		return cachedResourceName{name: "n", found: true}
	})

	const goroutines, rounds, ids = 32, 200, 5
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < rounds; j++ {
				if name, found := cache.lookup(j % ids); name != "n" || !found {
					t.Errorf("lookup(%d) = %q, %v", j%ids, name, found)
					return
				}
			}
		}()
	}
	wg.Wait()

	// 允许并发首查时的重复回源，但绝不能是"每次调用都查库"。
	if got := calls.Load(); got > ids*goroutines {
		t.Fatalf("load called %d times for %d lookups; cache is not effective", got, goroutines*rounds)
	}
}
