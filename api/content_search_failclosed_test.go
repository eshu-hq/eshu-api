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

// W4 Content search fail-closed and resilience proof (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#111). The terminated
// reader and the wave gate are both fail-closed: nil core answers the
// permanent local 503 (never Retry-After), an empty configured key
// answers the standing 401, a dead core answers the permanent 503
// backend_unavailable with exactly one dispatch attempt (no retry on a
// read), and cancellation propagates without retry. The deny shape
// lives in content_search_auth_test.go:
// TestContentSearchDenyBranchShapePinned.

// TestContentSearchTerminatedAs_NilCore503 pins that the native routes
// with no core origin configured answer the permanent local 503 in
// both negotiated shapes, with zero upstream contact.
func TestContentSearchTerminatedAs_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)

	for _, path := range contentSearchConcretePaths {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := "POST " + path + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := contentAuthedPost(path, contentSearchParityBodies[path][0], accept)
				rec := httptest.NewRecorder()
				srv.serveContentSearchTerminatedAs(rec, req, true)

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

// TestContentSearchTerminatedAs_Deny403 pins the unreachable deny
// branch through the terminated reader: a non-AllScopes identity is
// denied with the documented family 403.
func TestContentSearchTerminatedAs_Deny403(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, path := range contentSearchConcretePaths {
		t.Run("POST "+path, func(t *testing.T) {
			t.Parallel()

			req := contentAuthedPost(path, contentSearchParityBodies[path][0], "application/json")
			rec := httptest.NewRecorder()
			srv.serveContentSearchTerminatedAs(rec, req, false)

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want the documented 403", rec.Code)
			}
		})
	}
}

// TestContentSearchReadGate_EmptyKey401 pins the wave-gate counterpart:
// an empty configured key answers the standing dest 401 for every wave
// route, with zero core contact.
func TestContentSearchReadGate_EmptyKey401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, "")

	for _, path := range contentSearchConcretePaths {
		t.Run("POST "+path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(contentSearchParityBodies[path][0]))
			rec := httptest.NewRecorder()
			srv.Handler(DefaultV0Sunset).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (gate denies with zero core contact)", core.hits)
			}
		})
	}
}

// TestContentSearchRelayExplicit_NilCore503 pins that the explicit
// relay leg fails closed too: a foreign-class caller with no core
// origin configured answers the permanent local 503, never a
// dest-invented credential error.
func TestContentSearchRelayExplicit_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)

	for _, path := range contentSearchConcretePaths {
		t.Run("POST "+path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(contentSearchParityBodies[path][0]))
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

// TestContentSearchNative_DeadCoreIs503WithoutRetry pins dead-core
// behavior on the native routes: permanent 503 backend_unavailable
// with no Retry-After, in both negotiated shapes, with the attempt
// count proving exactly one dispatch attempt (no retry on a read).
func TestContentSearchNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, path := range contentSearchConcretePaths {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := "POST " + path + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
				var attempts int32
				srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
				handler := srv.Handler(DefaultV0Sunset)

				req := authedRequest(http.MethodPost, path)
				req.Header.Set("Accept", accept)
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

// TestContentSearchNative_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native routes: a canceled caller
// gets the permanent 503 backend_unavailable with no retry, pre-cancel
// and mid-flight, with the attempt count proving a single dispatch
// attempt in both cases.
func TestContentSearchNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cancel string // "before" serves a pre-canceled context; "during" cancels mid-flight
	}{
		{"pre-cancel", "before"},
		{"mid-flight", "during"},
	}
	for _, tc := range cases {
		for _, path := range contentSearchConcretePaths {
			t.Run(tc.name+" POST "+path, func(t *testing.T) {
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
				req := authedRequest(http.MethodPost, path).WithContext(ctx)
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

// TestContentSearchNative_ParallelProbe pins that concurrent callers
// each get one terminated read: 16 parallel POSTs across the four legs
// answer 200 with the relayed core bytes and land 16 upstream hits (one
// forward per caller, no dest coalescing and no dest retry).
func TestContentSearchNative_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"results":[],"count":0}`)
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
			path := contentSearchConcretePaths[i%len(contentSearchConcretePaths)]
			req := authedRequest(http.MethodPost, path)
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
