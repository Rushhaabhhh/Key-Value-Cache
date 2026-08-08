package main

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

func TestNextPowerOf2(t *testing.T) {
	cases := map[int]int{0: 1, 1: 1, 2: 2, 3: 4, 4: 4, 5: 8, 16: 16, 17: 32}
	for in, want := range cases {
		if got := nextPowerOf2(in); got != want {
			t.Errorf("nextPowerOf2(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestShardedCachePutGet(t *testing.T) {
	c := NewShardedCache(4, 100)
	if _, ok := c.Get("missing"); ok {
		t.Fatal("expected miss on empty cache")
	}
	c.Put("a", "1")
	c.Put("b", "2")
	if v, ok := c.Get("a"); !ok || v != "1" {
		t.Fatalf("Get(a) = %q, %v, want 1, true", v, ok)
	}
	c.Put("a", "updated")
	if v, ok := c.Get("a"); !ok || v != "updated" {
		t.Fatalf("Get(a) after update = %q, %v, want updated, true", v, ok)
	}
	if got := c.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
}

func TestCacheShardLRUEviction(t *testing.T) {
	cs := newCacheShard(3)
	cs.Put("a", "1")
	cs.Put("b", "2")
	cs.Put("c", "3")
	// touch "a" so it becomes most-recently-used; "b" is now the LRU victim.
	cs.Get("a")
	cs.Put("d", "4")

	if _, ok := cs.Get("b"); ok {
		t.Fatal("expected b to be evicted as least-recently-used")
	}
	for _, k := range []string{"a", "c", "d"} {
		if _, ok := cs.Get(k); !ok {
			t.Fatalf("expected %s to still be present", k)
		}
	}
	if got := cs.Len(); got != 3 {
		t.Fatalf("Len() = %d, want capacity 3", got)
	}
}

func TestShardedCacheConcurrent(t *testing.T) {
	c := NewShardedCache(8, 10000)
	var wg sync.WaitGroup
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				k := fmt.Sprintf("g%d-k%d", g, i)
				c.Put(k, k)
				if v, ok := c.Get(k); !ok || v != k {
					t.Errorf("Get(%s) = %q, %v", k, v, ok)
				}
			}
		}(g)
	}
	wg.Wait()
}

func newTestCtx(method, path, body string) *fasthttp.RequestCtx {
	var ctx fasthttp.RequestCtx
	var req fasthttp.Request
	req.Header.SetMethod(method)
	req.SetRequestURI(path)
	req.SetBodyString(body)
	ctx.Init(&req, nil, nil)
	return &ctx
}

func TestHandlePutAndGet(t *testing.T) {
	cache = NewShardedCache(4, 1000)
	maxKVSize = 256

	ctx := newTestCtx("POST", "/put", `{"key":"name","value":"kv-cache"}`)
	handlePut(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("PUT status = %d, want 200; body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}

	ctx2 := newTestCtx("GET", "/get?key=name", "")
	handleGet(ctx2)
	if got := string(ctx2.Response.Body()); got != `{"status":"OK","key":"name","value":"kv-cache"}` {
		t.Fatalf("GET body = %s", got)
	}
}

func TestHandlePutValidation(t *testing.T) {
	cache = NewShardedCache(4, 1000)
	maxKVSize = 4

	ctx := newTestCtx("POST", "/put", `{"key":"toolongkey","value":"v"}`)
	handlePut(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("expected 400 for oversized key, got %d", ctx.Response.StatusCode())
	}

	ctx2 := newTestCtx("POST", "/put", `not-json`)
	handlePut(ctx2)
	if ctx2.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", ctx2.Response.StatusCode())
	}
}

func TestWriteJSONStringEscaping(t *testing.T) {
	buf := bytebufferpool.Get()
	writeJSONString(buf, "he said \"hi\"\n\t\\end")
	want := `"he said \"hi\"\n\t\\end"`
	if got := buf.String(); got != want {
		t.Fatalf("writeJSONString = %s, want %s", got, want)
	}
}

// -- benchmarks ---------------------------------------------------------

func BenchmarkCachePutGetParallel(b *testing.B) {
	c := NewShardedCache(defaultShardCount(), 2_000_000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			k := "key-" + strconv.Itoa(i%100000)
			c.Put(k, "value-payload")
			c.Get(k)
			i++
		}
	})
}

func BenchmarkCacheGetParallel(b *testing.B) {
	c := NewShardedCache(defaultShardCount(), 2_000_000)
	for i := 0; i < 100000; i++ {
		k := "key-" + strconv.Itoa(i)
		c.Put(k, "value-payload")
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get("key-" + strconv.Itoa(i%100000))
			i++
		}
	})
}

func BenchmarkHandlePut(b *testing.B) {
	cache = NewShardedCache(defaultShardCount(), 2_000_000)
	maxKVSize = 256
	body := `{"key":"benchkey","value":"benchvalue-payload"}`
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := newTestCtx("POST", "/put", body)
			handlePut(ctx)
		}
	})
}

func BenchmarkHandleGet(b *testing.B) {
	cache = NewShardedCache(defaultShardCount(), 2_000_000)
	maxKVSize = 256
	cache.Put("benchkey", "benchvalue-payload")
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := newTestCtx("GET", "/get?key=benchkey", "")
			handleGet(ctx)
		}
	})
}
