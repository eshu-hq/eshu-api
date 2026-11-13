// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Drift fail-closed and resilience proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#117). The terminated reader and the wave
// gate are both fail-closed: nil core answers the permanent local 503
// (never Retry-After), an empty configured key answers the standing
// 401, a dead core answers the permanent 503 backend_unavailable with
// exactly one dispatch attempt (no retry on a read), and cancellation
// propagates without retry. The deny shape lives in
// drift_auth_test.go: TestDriftDenyBranchShapePinned.

// TestDriftTerminatedAs_NilCore503 pins that the native routes with no
// core origin configured answer the permanent local 503 in both
// negotiated shapes, with zero upstream contact.
func TestDriftTerminatedAs_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)

	calls := []struct {
		name string
		call func(accept string) *http.Request
	}{
		{"POST findings", func(accept string) *http.Request { return driftAuthedPost(driftFindingsBodies[0], accept) }},
		{"GET packet", func(accept string) *http.Request { return driftAuthedGet(driftPacketTargets[0], accept) }},
	}
	for _, leg := range calls {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.name + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := leg.call(accept)
				rec := httptest.NewRecorder()
				srv.serveDriftTerminatedAs(rec, req, true)

				res := rec.Result()
				if res.StatusCode != http.StatusServiceUnavailable {
					t.Fatalf("status = %d, want the permanent local 503", res.StatusCode)
				}
				if res.Header.Get("Retry-After") != "" {
					t.Errorf("Retry-After = %q, want empty (local 503 never carries it)", res.Header.Get("Retry-After"))
				}
			})
		}
	}
}

// TestDriftRelayExplicit_NilCore503 pins the fail-closed relay: an
// unresolvable credential class with no core origin configured answers
// the permanent local 503 on the relay lane (never Retry-After).
func TestDriftRelayExplicit_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)

	for _, leg := range []string{"POST findings", "GET packet"} {
		t.Run(leg, func(t *testing.T) {
			t.Parallel()

			var req *http.Request
			if strings.HasPrefix(leg, "POST") {
				req = httptest.NewRequest(http.MethodPost, driftFindingsPath, strings.NewReader(driftFindingsBodies[0]))
			} else {
				req = httptest.NewRequest(http.MethodGet, driftPacketTargets[0], nil)
			}
			req.Header.Set("Authorization", "Bearer [REDACTED]")
			rec := httptest.NewRecorder()
			srv.Handler(DefaultV0Sunset).ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want the permanent local 503", rec.Code)
			}
			if rec.Header().Get("Retry-After") != "" {
				t.Errorf("Retry-After = %q, want empty (local 503 never carries it)", rec.Header().Get("Retry-After"))
			}
		})
	}
}

// TestDriftNative_DeadCoreIs503WithoutRetry pins dead-core behavior
// on the native routes: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count
// proving exactly one dispatch attempt (no retry on a read).
func TestDriftNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	calls := []struct {
		name string
		call func(accept string) *http.Request
	}{
		{"POST findings", func(accept string) *http.Request {
			req := authedRequest(http.MethodPost, driftFindingsPath)
			req.Header.Set("Accept", accept)
			return req
		}},
		{"GET packet", func(accept string) *http.Request {
			return driftAuthedGet(driftPacketTargets[0], accept)
		}},
	}
	for _, leg := range calls {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.name + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
				var attempts int32
				srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
				handler := srv.Handler(DefaultV0Sunset)

				req := leg.call(accept)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if got := atomic.LoadInt32(&attempts); got != 1 {
					t.Errorf("dispatch attempts = %d, want exactly 1 (no retry)", got)
				}
				if rec.Code != http.StatusServiceUnavailable {
					t.Errorf("status = %d, want 503", rec.Code)
				}
				if got := rec.Header().Get("Retry-After"); got != "" {
					t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if !strings.Contains(string(body), "backend_unavailable") {
					t.Errorf("body = %q, want backend_unavailable", body)
				}
			})
		}
	}
}

// TestDriftNative_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native routes: a canceled caller
// gets the permanent 503 backend_unavailable with no retry, pre-cancel
// and mid-flight, with the attempt count proving a single dispatch
// attempt in both cases.
func TestDriftNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cancel string // "before" serves a pre-canceled context; "during" cancels mid-flight
	}{
		{"pre-cancel", "before"},
		{"mid-flight", "during"},
	}
	for _, tc := range cases {
		for _, leg := range []string{"POST findings", "GET packet"} {
			t.Run(tc.name+" "+leg, func(t *testing.T) {
				t.Parallel()

				slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					time.Sleep(500 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
				}))
				defer slow.Close()

				srv := NewServer(slow.URL, skeletonTestKey)
				var attempts int32
				srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
				handler := srv.Handler(DefaultV0Sunset)

				ctx, cancel := context.WithCancel(context.Background())
				if tc.cancel == "before" {
					cancel()
				} else {
					defer cancel()
					go func() {
						time.Sleep(100 * time.Millisecond)
						cancel()
					}()
				}
				var req *http.Request
				if strings.HasPrefix(leg, "POST") {
					req = authedRequest(http.MethodPost, driftFindingsPath).WithContext(ctx)
				} else {
					req = authedRequest(http.MethodGet, driftPacketTargets[0]).WithContext(ctx)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if got := atomic.LoadInt32(&attempts); got != 1 {
					t.Errorf("dispatch attempts = %d, want exactly 1 (no retry)", got)
				}
				if rec.Code != http.StatusServiceUnavailable {
					t.Errorf("status = %d, want 503", rec.Code)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if !strings.Contains(string(body), "backend_unavailable") {
					t.Errorf("body = %q, want backend_unavailable", body)
				}
			})
		}
	}
}

// TestDriftNative_ParallelProbe pins that concurrent callers each get
// one terminated read: 16 parallel calls across the two legs answer
// 200 with the relayed core bytes and land 16 upstream hits (one
// forward per caller, no dest coalescing and no dest retry).
func TestDriftNative_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"drift_findings":[],"findings_count":0}`)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	const callers = 16
	var wg sync.WaitGroup
	failures := make([]string, callers)
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var req *http.Request
			path := driftFindingsPath
			if i%2 == 0 {
				req = authedRequest(http.MethodPost, driftFindingsPath)
			} else {
				path = driftPacketPath
				req = authedRequest(http.MethodGet, driftPacketTargets[0])
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = path
			}
		}(i)
	}
	wg.Wait()

	for i, path := range failures {
		if path != "" {
			t.Errorf("caller %d %q: non-200 answer", i, path)
		}
	}
	if got := hits.Load(); got != callers {
		t.Errorf("upstream hits = %d, want %d (one forward per caller)", got, callers)
	}
}
