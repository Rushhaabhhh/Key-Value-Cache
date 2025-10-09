package main

import (
	"encoding/json"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

const (
	maxKeyValueSize = 256
	numShards       = 4
)

type CacheShard struct {
	sync.RWMutex
	items map[string]string
	count int
}

func NewCacheShard() *CacheShard {
	return &CacheShard{
		items: make(map[string]string, 700000),
		count: 0,
	}
}

func (cs *CacheShard) Put(key, value string) {
	cs.Lock()
	if cs.count >= 700000 {
		log.Println("Cache is full, removing an item")
		for k := range cs.items {
			delete(cs.items, k)
			cs.count--
			break
		}
	}
	if _, exists := cs.items[key]; !exists {
		cs.count++
	}
	cs.items[key] = value
	cs.Unlock()
}

func (cs *CacheShard) Get(key string) (string, bool) {
	cs.RLock()
	val, ok := cs.items[key]
	cs.RUnlock()
	return val, ok
}

type ShardedCache struct {
	shards    []*CacheShard
	shardMask uint64
}

func NewShardedCache(numShards int) *ShardedCache {
	powerOf2 := 1
	for powerOf2 < numShards {
		powerOf2 *= 2
	}
	sc := &ShardedCache{
		shards:    make([]*CacheShard, powerOf2),
		shardMask: uint64(powerOf2 - 1),
	}
	for i := 0; i < powerOf2; i++ {
		sc.shards[i] = NewCacheShard()
	}
	return sc
}

func djb2Hash(s string) uint64 {
	var hash uint64 = 5381
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) + hash) + uint64(s[i])
	}
	return hash
}

func (sc *ShardedCache) Put(key, value string) {
	shard := sc.shards[djb2Hash(key)&sc.shardMask]
	shard.Put(key, value)
}

func (sc *ShardedCache) Get(key string) (string, bool) {
	shard := sc.shards[djb2Hash(key)&sc.shardMask]
	return shard.Get(key)
}

type PutRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type GetResponse struct {
	Status string `json:"status"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

var (
	putSuccessBytes  = []byte(`{"status":"OK","message":"Key inserted/updated successfully."}`)
	keyNotFoundBytes = []byte(`{"status":"ERROR","message":"Key not found."}`)
)

var cache *ShardedCache

func requestHandler(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Method()) {
	case "POST":
		if string(ctx.Path()) == "/put" {
			body := ctx.PostBody()
			var req PutRequest
			if err := json.Unmarshal(body, &req); err != nil {
				ctx.Error("Bad request", fasthttp.StatusBadRequest)
				return
			}
			if req.Key == "" {
				ctx.Error("Key parameter is required", fasthttp.StatusBadRequest)
				return
			}
			if len(req.Key) > maxKeyValueSize || len(req.Value) > maxKeyValueSize {
				ctx.Error("Bad request", fasthttp.StatusBadRequest)
				return
			}
			cache.Put(req.Key, req.Value)
			ctx.Response.Header.Set("Content-Type", "application/json")
			ctx.SetBody(putSuccessBytes)
			return
		}
	case "GET":
		if string(ctx.Path()) == "/get" {
			key := string(ctx.QueryArgs().Peek("key"))
			if key == "" {
				ctx.Error("Key parameter is required", fasthttp.StatusBadRequest)
				return
			}
			value, found := cache.Get(key)
			if !found {
				ctx.Response.Header.Set("Content-Type", "application/json")
				ctx.SetBody(keyNotFoundBytes)
				return
			}
			resp := GetResponse{
				Status: "OK",
				Key:    key,
				Value:  value,
			}
			respJSON, err := json.Marshal(resp)
			if err != nil {
				ctx.Error("Bad request", fasthttp.StatusBadRequest)
				return
			}
			ctx.Response.Header.Set("Content-Type", "application/json")
			ctx.SetBody(respJSON)
			return
		}
	}
	ctx.Error("Unsupported request", fasthttp.StatusNotFound)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	cache = NewShardedCache(numShards)

	s := &fasthttp.Server{
		Handler:              requestHandler,
		Name:                 "UltraFastKVCache",
		ReadTimeout:          5 * time.Second,
		WriteTimeout:         5 * time.Second,
		MaxConnsPerIP:        0,
		MaxKeepaliveDuration: 60 * time.Second,
	}
	log.Printf("Starting fasthttp server on %s", ":7171")
	if err := s.ListenAndServe(":7171"); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}