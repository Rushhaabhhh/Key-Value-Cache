package main

import (
	"container/list"
	"context"
	"encoding/json"
	"hash/maphash"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
	"go.uber.org/automaxprocs/maxprocs"
)

const contentTypeJSON = "application/json"

// ---------------------------------------------------------------------------
// Configuration (all overridable via environment variables so the same binary
// behaves well on a laptop, a shared VM, or a CPU-limited container without
// code changes).
// ---------------------------------------------------------------------------

type Config struct {
	Host               string
	Port               string
	ShardCountOverride int // 0 = auto-derive from GOMAXPROCS
	CacheCapacity      int // total items across all shards
	MaxKVSize          int // max bytes for key or value
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	ShutdownTimeout    time.Duration
}

func (c Config) Addr() string { return c.Host + ":" + c.Port }

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		log.Printf("config: invalid value %s=%q, using default %d", key, v, def)
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
		log.Printf("config: invalid value %s=%q, using default %s", key, v, def)
	}
	return def
}

func loadConfig() Config {
	return Config{
		Host:               envString("HOST", ""),
		Port:               envString("PORT", "7171"),
		ShardCountOverride: envInt("SHARD_COUNT", 0),
		CacheCapacity:      envInt("CACHE_CAPACITY", 2_800_000),
		MaxKVSize:          envInt("MAX_KV_SIZE", 256),
		ReadTimeout:        envDuration("READ_TIMEOUT_SECONDS", 5*time.Second),
		WriteTimeout:       envDuration("WRITE_TIMEOUT_SECONDS", 5*time.Second),
		IdleTimeout:        envDuration("IDLE_TIMEOUT_SECONDS", 60*time.Second),
		ShutdownTimeout:    envDuration("SHUTDOWN_TIMEOUT_SECONDS", 10*time.Second),
	}
}

// defaultShardCount scales shard count with the CPU quota actually granted to
// this process (post GOMAXPROCS auto-tuning), not the physical host's core
// count. That keeps lock contention low on big machines while staying sane
// inside a 1-2 vCPU container. Shard count must stay a power of two so the
// hot path can select a shard with a bitmask instead of a division.
func defaultShardCount() int {
	n := runtime.GOMAXPROCS(0) * 4
	if n < 16 {
		n = 16
	}
	if n > 1024 {
		n = 1024
	}
	return nextPowerOf2(n)
}

func nextPowerOf2(n int) int {
	if n < 1 {
		return 1
	}
	p := 1
	for p < n {
		p *= 2
	}
	return p
}

// ---------------------------------------------------------------------------
// Cache: a sharded, fixed-capacity, LRU-evicting key-value store.
//
// Keys are routed to shards with hash/maphash, which is both fast (AES
// instructions on modern CPUs) and randomly seeded per-process, so an
// attacker cannot craft keys that all collide onto one shard to create a
// lock-contention denial of service (a real weakness of fixed hashes like
// djb2/FNV).
//
// Each shard keeps a map + doubly linked list to get true O(1) LRU
// eviction instead of evicting an arbitrary map-iteration-order key.
// ---------------------------------------------------------------------------

type entry struct {
	key   string
	value string
}

type CacheShard struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	order    *list.List // front = most recently used
}

func newCacheShard(capacity int) *CacheShard {
	if capacity < 1 {
		capacity = 1
	}
	return &CacheShard{
		capacity: capacity,
		items:    make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

func (cs *CacheShard) Put(key, value string) {
	cs.mu.Lock()
	if el, ok := cs.items[key]; ok {
		el.Value.(*entry).value = value
		cs.order.MoveToFront(el)
		cs.mu.Unlock()
		return
	}
	if cs.order.Len() >= cs.capacity {
		if back := cs.order.Back(); back != nil {
			cs.order.Remove(back)
			delete(cs.items, back.Value.(*entry).key)
		}
	}
	cs.items[key] = cs.order.PushFront(&entry{key: key, value: value})
	cs.mu.Unlock()
}

func (cs *CacheShard) Get(key string) (string, bool) {
	cs.mu.Lock()
	el, ok := cs.items[key]
	if !ok {
		cs.mu.Unlock()
		return "", false
	}
	cs.order.MoveToFront(el)
	val := el.Value.(*entry).value
	cs.mu.Unlock()
	return val, true
}

func (cs *CacheShard) Len() int {
	cs.mu.Lock()
	n := cs.order.Len()
	cs.mu.Unlock()
	return n
}

type ShardedCache struct {
	shards    []*CacheShard
	shardMask uint64
	seed      maphash.Seed
}

func NewShardedCache(numShards, totalCapacity int) *ShardedCache {
	numShards = nextPowerOf2(numShards)
	perShard := totalCapacity / numShards
	if perShard < 1 {
		perShard = 1
	}
	sc := &ShardedCache{
		shards:    make([]*CacheShard, numShards),
		shardMask: uint64(numShards - 1),
		seed:      maphash.MakeSeed(),
	}
	for i := range sc.shards {
		sc.shards[i] = newCacheShard(perShard)
	}
	return sc
}

func (sc *ShardedCache) shardFor(key string) *CacheShard {
	return sc.shards[maphash.String(sc.seed, key)&sc.shardMask]
}

func (sc *ShardedCache) Put(key, value string) { sc.shardFor(key).Put(key, value) }

func (sc *ShardedCache) Get(key string) (string, bool) { return sc.shardFor(key).Get(key) }

func (sc *ShardedCache) Len() int {
	total := 0
	for _, s := range sc.shards {
		total += s.Len()
	}
	return total
}

// ---------------------------------------------------------------------------
// HTTP layer
// ---------------------------------------------------------------------------

type PutRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var putReqPool = sync.Pool{New: func() any { return new(PutRequest) }}

var (
	putSuccessBytes  = []byte(`{"status":"OK","message":"Key inserted/updated successfully."}`)
	keyNotFoundBytes = []byte(`{"status":"ERROR","message":"Key not found."}`)
	healthOKBytes    = []byte(`{"status":"OK"}`)
)

var (
	cache     *ShardedCache
	maxKVSize int
)

const hexDigits = "0123456789abcdef"

// writeJSONString appends a properly escaped JSON string to buf. Hand-rolled
// (instead of encoding/json.Marshal) to avoid reflection on the hot response
// path; the happy path (no characters needing escaping) is a single
// allocation-free byte copy.
func writeJSONString(buf *bytebufferpool.ByteBuffer, s string) {
	buf.WriteByte('"')
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' {
			continue
		}
		if start < i {
			buf.WriteString(s[start:i])
		}
		switch c {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			buf.WriteString(`\u00`)
			buf.WriteByte(hexDigits[c>>4])
			buf.WriteByte(hexDigits[c&0xF])
		}
		start = i + 1
	}
	if start < len(s) {
		buf.WriteString(s[start:])
	}
	buf.WriteByte('"')
}

func writeError(ctx *fasthttp.RequestCtx, status int, message string) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	buf.WriteString(`{"status":"ERROR","message":`)
	writeJSONString(buf, message)
	buf.WriteByte('}')
	ctx.SetContentType(contentTypeJSON)
	ctx.SetStatusCode(status)
	ctx.SetBody(buf.B)
}

func handlePut(ctx *fasthttp.RequestCtx) {
	req := putReqPool.Get().(*PutRequest)
	req.Key, req.Value = "", ""
	defer putReqPool.Put(req)

	if err := json.Unmarshal(ctx.PostBody(), req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "Malformed JSON body")
		return
	}
	if req.Key == "" {
		writeError(ctx, fasthttp.StatusBadRequest, "Key parameter is required")
		return
	}
	if len(req.Key) > maxKVSize || len(req.Value) > maxKVSize {
		writeError(ctx, fasthttp.StatusBadRequest, "Key or value exceeds maximum size")
		return
	}
	cache.Put(req.Key, req.Value)
	ctx.SetContentType(contentTypeJSON)
	ctx.SetBody(putSuccessBytes)
}

func handleGet(ctx *fasthttp.RequestCtx) {
	key := string(ctx.QueryArgs().Peek("key"))
	if key == "" {
		writeError(ctx, fasthttp.StatusBadRequest, "Key parameter is required")
		return
	}
	value, found := cache.Get(key)
	ctx.SetContentType(contentTypeJSON)
	if !found {
		ctx.SetBody(keyNotFoundBytes)
		return
	}
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	buf.WriteString(`{"status":"OK","key":`)
	writeJSONString(buf, key)
	buf.WriteString(`,"value":`)
	writeJSONString(buf, value)
	buf.WriteByte('}')
	ctx.SetBody(buf.B)
}

func handleStats(ctx *fasthttp.RequestCtx) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	buf.WriteString(`{"status":"OK","shards":`)
	buf.B = strconv.AppendInt(buf.B, int64(len(cache.shards)), 10)
	buf.WriteString(`,"keys":`)
	buf.B = strconv.AppendInt(buf.B, int64(cache.Len()), 10)
	buf.WriteString(`,"goroutines":`)
	buf.B = strconv.AppendInt(buf.B, int64(runtime.NumGoroutine()), 10)
	buf.WriteString(`,"gomaxprocs":`)
	buf.B = strconv.AppendInt(buf.B, int64(runtime.GOMAXPROCS(0)), 10)
	buf.WriteByte('}')
	ctx.SetContentType(contentTypeJSON)
	ctx.SetBody(buf.B)
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Path()) {
	case "/put":
		if !ctx.IsPost() {
			writeError(ctx, fasthttp.StatusMethodNotAllowed, "POST required")
			return
		}
		handlePut(ctx)
	case "/get":
		if !ctx.IsGet() {
			writeError(ctx, fasthttp.StatusMethodNotAllowed, "GET required")
			return
		}
		handleGet(ctx)
	case "/health":
		ctx.SetContentType(contentTypeJSON)
		ctx.SetBody(healthOKBytes)
	case "/stats":
		handleStats(ctx)
	default:
		writeError(ctx, fasthttp.StatusNotFound, "Unsupported request")
	}
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	cfg := loadConfig()
	maxKVSize = cfg.MaxKVSize

	// Respect cgroup CPU quotas (Docker --cpus, Kubernetes limits) instead of
	// blindly trusting the host's full core count. Without this, a container
	// capped at e.g. 2 CPUs on a 32-core host would still set GOMAXPROCS=32,
	// causing excessive scheduling overhead and worse tail latency.
	if _, err := maxprocs.Set(maxprocs.Logger(log.Printf)); err != nil {
		log.Printf("maxprocs: could not adjust GOMAXPROCS: %v", err)
	}

	numShards := cfg.ShardCountOverride
	if numShards <= 0 {
		numShards = defaultShardCount()
	}
	cache = NewShardedCache(numShards, cfg.CacheCapacity)

	log.Printf("kv-cache starting: addr=%s shards=%d capacity=%d maxKVSize=%d GOMAXPROCS=%d",
		cfg.Addr(), len(cache.shards), cfg.CacheCapacity, cfg.MaxKVSize, runtime.GOMAXPROCS(0))

	server := &fasthttp.Server{
		Handler:              requestHandler,
		Name:                 "kv-cache",
		ReadTimeout:          cfg.ReadTimeout,
		WriteTimeout:         cfg.WriteTimeout,
		MaxConnsPerIP:        0,
		MaxKeepaliveDuration: cfg.IdleTimeout,
		MaxRequestBodySize:   cfg.MaxKVSize*4 + 1024,
		TCPKeepalive:         true,
		CloseOnShutdown:      true,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(cfg.Addr()); err != nil {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Fatalf("server error: %v", err)
	case sig := <-sigCh:
		log.Printf("received signal %s, shutting down gracefully (timeout %s)...", sig, cfg.ShutdownTimeout)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("error during shutdown: %v", err)
	}
	log.Println("server stopped")
}
