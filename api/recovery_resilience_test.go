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

// recoverySpellings returns every native spelling: each route as
// registered plus its trailing-slash twin.
func recoverySpellings() []struct {
	method string
	path   string
} {
	var out []struct {
		method string
		path   string
	}
	for _, tc := range recoveryNativeCases {
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

// TestRecoveryNative_UnconfiguredOriginFailsClosed pins the
// fail-closed origin: with no core origin configured every native
// spelling answers the permanent 503 backend_unavailable instead of
// panicking, in both negotiated shapes.
func TestRecoveryNative_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range recoverySpellings() {
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

// TestRecoveryNative_BothSpellingsWriteThrough pins that both
// spellings of every recovery path stay on the native route: method and
// path (as spelled) forward to the same core route.
func TestRecoveryNative_BothSpellingsWriteThrough(t *testing.T) {
	t.Parallel()

	for _, spelling := range recoverySpellings() {
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

// TestRecoveryNative_DeadCoreIs503WithoutRetry pins the dead-core
// shape on the native route: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count proving
// exactly one dispatch attempt (no retry on a write).
func TestRecoveryNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, "/api/v0/admin/replay")
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

// TestRecoveryNative_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native route: a canceled caller gets
// the permanent 503 backend_unavailable with no retry, pre-cancel and
// mid-flight, with the attempt count proving a single dispatch attempt
// in both cases. A long recovery (refinalize, recover-generations) that
// outruns the dispatch bound fails the same way: exactly one upstream
// attempt, no dest retry, and the core-side finalize still records the
// committed outcome for the idempotency-key retry to replay.
func TestRecoveryNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
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
			req := authedRequest(http.MethodPost, "/api/v0/admin/recover-generations").WithContext(ctx)
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

// TestRecoveryNative_ParallelProbe pins that concurrent callers each
// get one write-through: 16 parallel recover-generations POSTs answer
// 200 with the relayed core bytes and land 16 upstream hits (one
// forward per caller, no dest coalescing and no dest retry).
func TestRecoveryNative_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"recovered","enqueued":1,"scope_ids":["s1"],"idempotency_key":"k-parallel","duplicate":false}`)
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
			req := authedRequest(http.MethodPost, "/api/v0/admin/recover-generations")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = fmt.Sprintf("caller %d: status = %d, want 200", i, rec.Code)
				return
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), `"duplicate":false`) {
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

// TestRecoveryNative_V1AliasOrderingUnchanged pins that the v1 alias
// rewrite still lands the replay POST on its native handler with the v0
// deprecation headers absent.
func TestRecoveryNative_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"replayed","replayed_count":0,"work_item_ids":[],"idempotency_key":"k1","duplicate":false}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodPost, "/api/v1/admin/replay")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 || core.lastPath != "/api/v0/admin/replay" {
		t.Errorf("hits = %d path = %q, want alias rewrite to the native v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want relayed 200", rec.Code)
	}
}
