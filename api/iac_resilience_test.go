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

// iacSpellings returns every native spelling: each route as registered
// plus its trailing-slash twin.
func iacSpellings() []string {
	var out []string
	for _, tc := range iacNativeCases {
		path := iacRoutePath(tc.route)
		out = append(out, path, path+"/")
	}
	return out
}

// TestIaCNative_BothSpellingsForwardAsSpelled pins that every native
// spelling of every cleared leg stays on the native route: method and
// path (as spelled) forward to the same core route.
func TestIaCNative_BothSpellingsForwardAsSpelled(t *testing.T) {
	t.Parallel()

	for _, spelling := range iacSpellings() {
		core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, spelling)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != spelling {
			t.Errorf("%q: hits = %d lastPath = %q, want the native route to forward as spelled",
				spelling, core.hits, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("%q: status = %d, want relayed 200", spelling, rec.Code)
		}
	}
}

// TestIaCNative_DeadCoreIs503WithoutRetry pins the dead-core shape on
// the native routes: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count proving
// exactly one dispatch attempt (no retry on a read).
func TestIaCNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, tc := range iacNativeCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
			var attempts int32
			srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(http.MethodPost, iacRoutePath(tc.route))
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := atomic.LoadInt32(&attempts); got != 1 {
				t.Errorf("%s Accept %q: dispatch attempts = %d, want exactly 1 (no retry)", tc.name, accept, got)
			}
			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s Accept %q: status = %d, want 503", tc.name, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s Accept %q: Retry-After = %q, want absent (permanent 503)", tc.name, accept, got)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), "backend_unavailable") {
				t.Errorf("%s Accept %q: body = %q, want backend_unavailable", tc.name, accept, body)
			}
		}
	}
}

// TestIaCNative_CanceledContextFailsClosedWithoutRetry pins cancellation
// propagation on the native routes: a canceled caller gets the permanent
// 503 backend_unavailable with no retry, pre-cancel and mid-flight, with
// the attempt count proving a single dispatch attempt in both cases.
func TestIaCNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
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
			req := authedRequest(http.MethodPost, iacRoutePath(iacManagementStatusRoute)).WithContext(ctx)
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

// TestIaCNative_ParallelProbe pins that concurrent callers each get one
// write-through: 16 parallel POSTs across the four legs answer 200 with
// the relayed core bytes and land 16 upstream hits (one forward per
// caller, no dest coalescing and no dest retry).
func TestIaCNative_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":{"total":0},"error":null}`)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	paths := []string{
		iacRoutePath(iacUnmanagedResourcesRoute),
		iacRoutePath(iacManagementStatusRoute),
		iacRoutePath(iacManagementStatusExplainRoute),
		iacRoutePath(iacTerraformImportPlanCandidatesRoute),
		iacRoutePath(iacDeadRoute),
	}
	const callers = 16
	var wg sync.WaitGroup
	failures := make([]string, callers)
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := authedRequest(http.MethodPost, paths[i%len(paths)])
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = paths[i%len(paths)]
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
