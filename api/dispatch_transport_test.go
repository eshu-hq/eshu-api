// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Relay transport pool measurement (eshu-hq/eshu-api#131, part of
// eshu-hq/eshu#9669).
//
// Measurement-first: dispatchClient sets a timeout and a redirect rule
// but no Transport, so the relay uses http.DefaultTransport with
// DefaultMaxIdleConnsPerHost = 2 idle connections per host against the
// single core host. The theory (unproven) is that bursts wider than 2
// pay connection-setup latency under concurrent load. This file proves
// the pool behavior and measures the latency cost, if any: a stub core
// with a 50ms handler, 64 concurrent requests through the production
// Handler, new-connection counting via ConnState, p50/p99 latency.
//
// Stopping rule (#131): if the tuned transport shows no material
// latency improvement, the issue closes with the numbers and no
// production change lands.

// transportProbeWidth is the burst width: far above the default pool of
// 2, so pool exhaustion is deterministic, not statistical.
const transportProbeWidth = 64

// transportProbeDelay is the stub core handler delay: large enough that
// all 64 requests in a burst overlap in flight (goroutine launch
// stagger is microseconds), small enough that a burst costs ~50ms.
const transportProbeDelay = 50 * time.Millisecond

// transportProbeCore answers every request after transportProbeDelay
// with a fixed 200 body. Hits is atomic; the shared stubCore is not
// safe for 64-way concurrent use, so this probe keeps its own counter.
type transportProbeCore struct {
	hits *atomic.Int64
}

func (c *transportProbeCore) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		time.Sleep(transportProbeDelay)
		c.hits.Add(1)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
}

// transportProbeServer starts a stub core that counts new TCP
// connections via ConnState. It returns the server and a pointer to
// the StateNew counter; callers diff the counter across bursts.
func transportProbeServer(tb testing.TB, core *transportProbeCore) (*httptest.Server, *atomic.Int64) {
	tb.Helper()

	var newConns atomic.Int64
	upstream := httptest.NewUnstartedServer(core.handler())
	upstream.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConns.Add(1)
		}
	}
	upstream.Start()
	tb.Cleanup(upstream.Close)
	return upstream, &newConns
}

// relayBurstResult carries one burst's per-request latencies and the
// count of non-200 relay answers (want 0: the stub core always 200s).
type relayBurstResult struct {
	latencies []time.Duration
	badStatus int
}

// runRelayBurst fires width concurrent authenticated relay GETs through
// handler and returns their latencies. A start gate releases all
// goroutines together so the burst truly overlaps; each goroutine owns
// its request and recorder, so no request state is shared.
func runRelayBurst(handler http.Handler, width int) relayBurstResult {
	latencies := make([]time.Duration, width)
	var badStatus atomic.Int64
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < width; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			req := authedRequest(http.MethodGet, "/api/v0/repositories")
			rec := httptest.NewRecorder()
			begin := time.Now()
			handler.ServeHTTP(rec, req)
			latencies[i] = time.Since(begin)
			if rec.Code != http.StatusOK {
				badStatus.Add(1)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	return relayBurstResult{latencies: latencies, badStatus: int(badStatus.Load())}
}

// percentile reports the p-th percentile (p in [0,100]) of latencies.
// It sorts a copy, so callers keep their order.
func percentile(latencies []time.Duration, p float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	sorted := slices.Clone(latencies)
	slices.Sort(sorted)
	rank := int((p / 100) * float64(len(sorted)-1))
	return sorted[rank]
}

// tunedDispatchClient mirrors dispatchClient with an explicit idle pool
// of maxIdle connections per host, so the benchmark compares the
// default pool of 2 against a pool sized for the burst width.
func tunedDispatchClient(maxIdle int) *http.Client {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.MaxIdleConnsPerHost = maxIdle
	return &http.Client{
		Timeout:   dispatchTimeout,
		Transport: base,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// TestRelayTransportPoolCalibration proves the pool behavior the issue
// describes: a 64-way burst through the default client opens 64
// connections, and the next 64-way burst opens ~62 new ones because
// only 2 idle connections survived. It is intentionally not Parallel:
// it asserts absolute connection counts on its own stub core.
func TestRelayTransportPoolCalibration(t *testing.T) {
	var hits atomic.Int64
	upstream, newConns := transportProbeServer(t, &transportProbeCore{hits: &hits})

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	first := runRelayBurst(handler, transportProbeWidth)
	if first.badStatus != 0 {
		t.Fatalf("first burst non-200 answers = %d, want 0", first.badStatus)
	}
	firstNew := newConns.Load()

	second := runRelayBurst(handler, transportProbeWidth)
	if second.badStatus != 0 {
		t.Fatalf("second burst non-200 answers = %d, want 0", second.badStatus)
	}
	secondNew := newConns.Load() - firstNew

	if hits.Load() != 2*transportProbeWidth {
		t.Errorf("core hits = %d, want %d (every relayed request reaches core)", hits.Load(), 2*transportProbeWidth)
	}
	if firstNew != transportProbeWidth {
		t.Errorf("first-burst new connections = %d, want %d (cold pool opens one per in-flight request)", firstNew, transportProbeWidth)
	}
	// The pool keeps 2 idle per host, so the second burst reuses 2 and
	// opens ~62 new. The band tolerates scheduling jitter while still
	// proving the mechanism: far above 2, far from full reuse.
	if secondNew < 60 || secondNew > transportProbeWidth {
		t.Errorf("second-burst new connections = %d, want 60..%d (only the 2-idle pool survives)", secondNew, transportProbeWidth)
	}

	t.Logf("default pool: burst1 new=%d p50=%v p99=%v; burst2 new=%d p50=%v p99=%v",
		firstNew, percentile(first.latencies, 50), percentile(first.latencies, 99),
		secondNew, percentile(second.latencies, 50), percentile(second.latencies, 99))
}

// BenchmarkRelayTransportPool measures steady-state burst latency and
// connection churn with the default 2-idle pool versus a pool sized
// for the burst width. Each iteration is one 64-way burst; the first
// iteration warms the pool and the rest measure steady state. Run with:
//
//	go test ./api/ -run=NONE -bench=RelayTransportPool -benchtime=8x -count=1 -v
func BenchmarkRelayTransportPool(b *testing.B) {
	b.Run("default-pool-2", func(b *testing.B) {
		benchRelayBurst(b, dispatchClient())
	})
	b.Run("tuned-pool-64", func(b *testing.B) {
		benchRelayBurst(b, tunedDispatchClient(transportProbeWidth))
	})
}

// benchRelayBurst runs one burst per iteration on client and reports
// pooled p50/p99 latency plus average new connections per burst.
func benchRelayBurst(b *testing.B, client *http.Client) {
	b.Helper()

	var hits atomic.Int64
	upstream, newConns := transportProbeServer(b, &transportProbeCore{hits: &hits})

	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.httpClient = client
	handler := srv.Handler(DefaultV0Sunset)

	b.ReportAllocs()
	// One unmeasured burst warms the pool, so every measured iteration
	// is steady state: cold-start setup cost must not leak into either
	// leg of the comparison.
	_ = runRelayBurst(handler, transportProbeWidth)
	warmedNew := newConns.Load()
	b.ResetTimer()
	var all []time.Duration
	var bad int
	for i := 0; i < b.N; i++ {
		res := runRelayBurst(handler, transportProbeWidth)
		all = append(all, res.latencies...)
		bad += res.badStatus
	}
	b.StopTimer()
	if bad != 0 {
		b.Fatalf("non-200 answers = %d, want 0", bad)
	}
	b.ReportMetric(float64(percentile(all, 50))/float64(time.Millisecond), "p50-ms")
	b.ReportMetric(float64(percentile(all, 99))/float64(time.Millisecond), "p99-ms")
	b.ReportMetric(float64(newConns.Load()-warmedNew)/float64(b.N), "new-conns-per-burst")
}
