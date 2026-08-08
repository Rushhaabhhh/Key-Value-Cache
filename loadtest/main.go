// loadtest drives a running key-value-cache instance to its real, resident
// capacity and measures steady-state latency/throughput once it's there.
//
// It exists because ab/hey/wrk don't support unique per-request JSON bodies,
// which this workload needs (every write must be a new key to exercise real
// eviction pressure once the cache is full, and reads need to sample from
// what's actually been written).
//
// Capacity is detected, not assumed: rather than pre-computing how many keys
// equal "full" (fragile — it has to be kept in sync with the server's
// CACHE_CAPACITY, and even then shards don't all saturate in perfect
// lockstep), phase 1 writes unique keys as fast as it can and polls /stats
// until the resident key count stops growing. That's the actual signal the
// cache is full, whatever capacity it was started with.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type putReq struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type statsResp struct {
	Keys int64 `json:"keys"`
}

func main() {
	baseURL := flag.String("url", "http://127.0.0.1:7171", "server base url")
	fillTarget := flag.Int64("fill", 0, "unique keys to insert before measuring; 0 = auto-detect real capacity by filling until /stats stops growing (recommended)")
	fillConcurrency := flag.Int("fill-concurrency", 300, "concurrent writers during fill")
	maxFillDuration := flag.Duration("max-fill-duration", 5*time.Minute, "safety cap on auto-fill; aborts with a warning if the cache never plateaus (e.g. CACHE_CAPACITY set very large)")
	duration := flag.Duration("duration", 20*time.Second, "steady-state measurement duration, run AFTER the cache is confirmed full")
	testConcurrency := flag.Int("concurrency", 200, "concurrent workers during steady-state measurement")
	readRatio := flag.Float64("read-ratio", 0.9, "fraction of steady-state requests that are GET")
	logPath := flag.String("log", "loadtest_results.log", "output log file (also printed to stdout)")
	flag.Parse()

	f, err := os.Create(*logPath)
	if err != nil {
		log.Fatalf("cannot create log file: %v", err)
	}
	defer f.Close()
	logger := log.New(io.MultiWriter(os.Stdout, f), "", log.LstdFlags)

	tr := &http.Transport{
		MaxIdleConns:        2000,
		MaxIdleConnsPerHost: 2000,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: tr}

	logger.Printf("=== target server: %s ===", *baseURL)
	logStats(logger, client, *baseURL, "before fill")

	var written int64
	if *fillTarget > 0 {
		written = runFixedFill(logger, client, *baseURL, *fillTarget, *fillConcurrency)
	} else {
		written = runAutoFillUntilSaturated(logger, client, *baseURL, *fillConcurrency, *maxFillDuration)
	}
	logStats(logger, client, *baseURL, "after fill")
	if written <= 0 {
		logger.Fatalf("no keys were written during fill; is the server reachable at %s?", *baseURL)
	}

	// Reads must sample from keys likely to still be resident, not the whole
	// write history: auto-fill necessarily overshoots capacity (it keeps
	// writing until it *observes* the plateau), so the earliest fillkeys are
	// already evicted by the time Phase 2 starts. Sampling uniformly across
	// all of `written` would mostly hit already-evicted keys and report a
	// misleadingly low hit rate that has nothing to do with the cache's real
	// behavior — it would just reflect this tool's own overshoot.
	residentKeys, err := fetchKeyCount(client, *baseURL)
	if err != nil || residentKeys <= 0 {
		residentKeys = written
	}
	readWindowStart := written - residentKeys
	if readWindowStart < 0 {
		readWindowStart = 0
	}
	logger.Printf("Phase 2 will sample reads from the most recent %d written keys (indices %d..%d), matching what's actually resident",
		residentKeys, readWindowStart, written-1)

	// ---- Phase 2: timed mixed workload against the (now full) cache ----
	logger.Printf("Phase 2: steady-state mixed load (%.0f%% GET / %.0f%% PUT) for %s with %d workers...",
		*readRatio*100, (1-*readRatio)*100, *duration, *testConcurrency)

	deadline := time.Now().Add(*duration)
	var reqCount, errCount, hitCount, missCount int64
	var mu sync.Mutex
	var allLatencies []time.Duration

	var wg2 sync.WaitGroup
	testStart := time.Now()
	for w := 0; w < *testConcurrency; w++ {
		wg2.Add(1)
		go func(seed int64) {
			defer wg2.Done()
			rnd := rand.New(rand.NewSource(seed))
			local := make([]time.Duration, 0, 8192)
			buf := make([]byte, 512)
			for time.Now().Before(deadline) {
				t0 := time.Now()
				var resp *http.Response
				var err error
				isRead := rnd.Float64() < *readRatio
				if isRead {
					idx := readWindowStart + rnd.Int63n(written-readWindowStart)
					k := "fillkey-" + strconv.FormatInt(idx, 10)
					resp, err = client.Get(*baseURL + "/get?key=" + k)
				} else {
					k := "loadkey-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.Itoa(rnd.Int())
					body, _ := json.Marshal(putReq{Key: k, Value: "v"})
					resp, err = client.Post(*baseURL+"/put", "application/json", bytes.NewReader(body))
				}
				elapsed := time.Since(t0)
				if err != nil {
					atomic.AddInt64(&errCount, 1)
					continue
				}
				n, _ := io.ReadFull(resp.Body, buf)
				io.Copy(io.Discard, resp.Body) // drain any remainder
				resp.Body.Close()
				if isRead {
					if bytes.Contains(buf[:n], []byte(`"status":"OK"`)) {
						atomic.AddInt64(&hitCount, 1)
					} else {
						atomic.AddInt64(&missCount, 1)
					}
				}
				atomic.AddInt64(&reqCount, 1)
				local = append(local, elapsed)
			}
			mu.Lock()
			allLatencies = append(allLatencies, local...)
			mu.Unlock()
		}(int64(w) + 1)
	}
	wg2.Wait()
	testElapsed := time.Since(testStart)

	sort.Slice(allLatencies, func(i, j int) bool { return allLatencies[i] < allLatencies[j] })
	pct := func(p float64) time.Duration {
		if len(allLatencies) == 0 {
			return 0
		}
		idx := int(p * float64(len(allLatencies)))
		if idx >= len(allLatencies) {
			idx = len(allLatencies) - 1
		}
		return allLatencies[idx]
	}

	logger.Printf("Phase 2 done: %d requests in %s (%.0f req/s), %d errors",
		reqCount, testElapsed.Round(time.Millisecond), float64(reqCount)/testElapsed.Seconds(), errCount)
	if hitCount+missCount > 0 {
		logger.Printf("Read hit rate: %.2f%% (%d hits, %d misses — misses are keys evicted since being written, expected at capacity)",
			100*float64(hitCount)/float64(hitCount+missCount), hitCount, missCount)
	}
	if len(allLatencies) > 0 {
		logger.Printf("Latency: p50=%s p90=%s p95=%s p99=%s p999=%s max=%s",
			pct(0.50), pct(0.90), pct(0.95), pct(0.99), pct(0.999), allLatencies[len(allLatencies)-1])
	}
	logStats(logger, client, *baseURL, "after steady-state test")
}

// runFixedFill inserts exactly target unique keys and returns how many were
// written. Kept as an explicit opt-in (-fill N) for testing at a specific
// point below capacity; auto-detection (runAutoFillUntilSaturated) is the
// default and correct choice for "measure at max capacity."
func runFixedFill(logger *log.Logger, client *http.Client, baseURL string, target int64, concurrency int) int64 {
	logger.Printf("Phase 1: filling %d unique keys with %d concurrent workers (fixed count, not saturation-verified)...", target, concurrency)
	start := time.Now()
	var counter int64
	var errCount int64
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := atomic.AddInt64(&counter, 1) - 1
				if i >= target {
					return
				}
				k := "fillkey-" + strconv.FormatInt(i, 10)
				body, _ := json.Marshal(putReq{Key: k, Value: "value-" + strconv.FormatInt(i, 10)})
				resp, err := client.Post(baseURL+"/put", "application/json", bytes.NewReader(body))
				if err != nil {
					atomic.AddInt64(&errCount, 1)
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	logger.Printf("Phase 1 done: %d keys written in %s (%.0f req/s), %d errors",
		target, elapsed.Round(time.Millisecond), float64(target)/elapsed.Seconds(), errCount)
	return target
}

// runAutoFillUntilSaturated writes unique keys as fast as possible and polls
// /stats once a second; once the resident key count stops growing for three
// consecutive polls, the cache is genuinely full (further writes are purely
// evicting), so it stops and returns how many keys were written in total.
func runAutoFillUntilSaturated(logger *log.Logger, client *http.Client, baseURL string, concurrency int, maxDuration time.Duration) int64 {
	logger.Printf("Phase 1: auto-filling with %d concurrent writers until resident key count plateaus (max wait %s)...", concurrency, maxDuration)
	ctx, cancel := context.WithTimeout(context.Background(), maxDuration)
	defer cancel()

	var counter int64
	var wg sync.WaitGroup
	start := time.Now()
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				i := atomic.AddInt64(&counter, 1) - 1
				k := "fillkey-" + strconv.FormatInt(i, 10)
				body, _ := json.Marshal(putReq{Key: k, Value: "value-" + strconv.FormatInt(i, 10)})
				resp, err := client.Post(baseURL+"/put", "application/json", bytes.NewReader(body))
				if err != nil {
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}()
	}

	lastKeys := int64(-1)
	stablePolls := 0
	saturated := false
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
poll:
	for {
		select {
		case <-ctx.Done():
			logger.Printf("Phase 1: WARNING max-fill-duration reached before the cache plateaued — capacity may not have been reached; raise -max-fill-duration or lower CACHE_CAPACITY")
			break poll
		case <-ticker.C:
			keys, err := fetchKeyCount(client, baseURL)
			if err != nil {
				continue
			}
			if lastKeys >= 0 {
				delta := keys - lastKeys
				logger.Printf("Phase 1: resident keys=%d (+%d in last 1s)", keys, delta)
				if delta <= 0 {
					stablePolls++
				} else {
					stablePolls = 0
				}
				if stablePolls >= 3 {
					saturated = true
					break poll
				}
			}
			lastKeys = keys
		}
	}
	cancel()
	wg.Wait()
	elapsed := time.Since(start)
	written := atomic.LoadInt64(&counter)
	finalKeys, _ := fetchKeyCount(client, baseURL)
	if saturated {
		logger.Printf("Phase 1 done: cache SATURATED at %d resident keys (wrote %d keys total in %s, %.0f req/s)",
			finalKeys, written, elapsed.Round(time.Millisecond), float64(written)/elapsed.Seconds())
	} else {
		logger.Printf("Phase 1 done, NOT saturated: %d resident keys after writing %d keys in %s — results below reflect a partially-filled cache",
			finalKeys, written, elapsed.Round(time.Millisecond))
	}
	return written
}

func fetchKeyCount(client *http.Client, baseURL string) (int64, error) {
	resp, err := client.Get(baseURL + "/stats")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var s statsResp
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return 0, err
	}
	return s.Keys, nil
}

func logStats(logger *log.Logger, client *http.Client, baseURL, label string) {
	resp, err := client.Get(baseURL + "/stats")
	if err != nil {
		logger.Printf("stats (%s): error: %v", label, err)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	logger.Printf("stats (%s): %s", label, string(b))
}
