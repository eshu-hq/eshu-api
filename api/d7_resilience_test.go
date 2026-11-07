// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// D7 admin-four tier-C clone resilience proof (Refs eshu-hq/eshu#9669,
// part of #6707). The clone holds the same failure contract as the
// recovery lane: fail closed with no core configured, exactly one
// upstream attempt on every failure shape, and no dest retry on
// cancellation or an unstable core. Transport 500s/503s are retryable
// only end-to-end by the client, never by dest.

// d7Spellings returns every native spelling: each route as registered
// plus its trailing-slash twin.
func d7Spellings() []struct {
	method string
	path   string
} {
	var out []struct {
		method string
		path   string
	}
	for _, tc := range d7NativeCases {
		out = append(out, struct {
			method string
			path   string
		}{tc.method, tc.path}, struct {
			method string
			path   string
		}{tc.method, tc.path + "/"})
	}
	return out
}

// TestD7Native_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured every native spelling answers
// the permanent 503 backend_unavailable instead of panicking, in both
// negotiated shapes.
func TestD7Native_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range d7Spellings() {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(spelling.method, spelling.path)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("%s %q Accept %q: panicked with %v, want 503", spelling.method, spelling.path, accept, recovered)
					}
				}()
				handler.ServeHTTP(rec, req)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s %q Accept %q: status = %d, want 503", spelling.method, spelling.path, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s %q Accept %q: Retry-After = %q, want absent (permanent 503)", spelling.method, spelling.path, accept, got)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "backend_unavailable") {
				t.Errorf("%s %q Accept %q: body = %q, want backend_unavailable", spelling.method, spelling.path, accept, body)
			}
		}
	}
}

// TestD7Native_BothSpellingsWriteThrough pins that both spellings of
// every admin-four path stay on the native route: method and path (as
// spelled) forward to the same core route.
func TestD7Native_BothSpellingsWriteThrough(t *testing.T) {
	t.Parallel()

	for _, spelling := range d7Spellings() {
		core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(spelling.method, spelling.path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != spelling.path {
			t.Errorf("%s %q: hits = %d lastPath = %q, want the native route to forward as spelled",
				spelling.method, spelling.path, core.hits, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("%s %q: status = %d, want relayed 200", spelling.method, spelling.path, rec.Code)
		}
	}
}

// TestD7Native_DeadCoreIs503WithoutRetry pins the dead-core shape on
// the native route: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count proving
// exactly one dispatch attempt (no retry on a write). The unstable-core
// half of the tier-C proof box: a 500 or a retryable 503 from the core
// answers the same way — one forward, core bytes relayed — and only
// the client may retry end-to-end (covered in the write-through
// matrix, which pins exactly one upstream hit on those rows).
func TestD7Native_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, "/api/v0/admin/reindex")
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := atomic.LoadInt32(&attempts); got != 1 {
			t.Errorf("Accept %q: dispatch attempts = %d, want exactly 1 (no retry)", accept, got)
		}
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Accept %q: status = %d, want 503", accept, rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("Accept %q: Retry-After = %q, want absent (permanent 503)", accept, got)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "backend_unavailable") {
			t.Errorf("Accept %q: body = %q, want backend_unavailable", accept, body)
		}
	}
}

// TestD7Native_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native route: a canceled caller gets
// the permanent 503 backend_unavailable with no retry, pre-cancel and
// mid-flight, with the attempt count proving a single dispatch attempt
// in both cases.
func TestD7Native_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cancel string // "before" serves a pre-canceled context; "during" cancels mid-flight
	}{
		{"pre-cancel", "before"},
		{"mid-flight", "during"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			req := authedRequest(http.MethodPost, "/api/v0/admin/skip").WithContext(ctx)
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

// TestD7Native_ParallelProbe pins that concurrent callers each get one
// write-through: 16 parallel dead-letter POSTs answer 200 with the
// relayed core bytes and land 16 upstream hits (one forward per caller,
// no dest coalescing and no dest retry). Dest holds no ledger, so it
// serializes nothing — which is exactly the no-worse-than-core proof:
// the same burst against the core directly converges the same way.
func TestD7Native_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"dead_lettered","count":1,"idempotency_key":"k-parallel"}`)
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
			req := authedRequest(http.MethodPost, "/api/v0/admin/dead-letter")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = fmt.Sprintf("caller %d: status = %d, want 200", i, rec.Code)
				return
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), `"status":"dead_lettered"`) {
				failures[i] = fmt.Sprintf("caller %d: body = %q, want the relayed core bytes", i, body)
			}
		}(i)
	}
	wg.Wait()

	for _, failure := range failures {
		if failure != "" {
			t.Error(failure)
		}
	}
	if got := hits.Load(); got != callers {
		t.Errorf("upstream hits = %d, want %d (one write-through per caller, no retry)", got, callers)
	}
}

// TestD7Native_V1AliasOrderingUnchanged pins that the v1 alias rewrite
// still lands the backfill POST on its native handler with the v1
// deprecation headers absent.
func TestD7Native_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"recorded","request_id":"b1"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodPost, "/api/v1/admin/backfill")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 || core.lastPath != "/api/v0/admin/backfill" {
		t.Errorf("hits = %d path = %q, want alias rewrite to the native v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want relayed 200", rec.Code)
	}
}
