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

// Code-divergence fail-closed and resilience proof (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#125). The terminated
// reader and the wave gate are both fail-closed: nil core answers
// the permanent local 503 (never Retry-After), an empty configured
// key answers the standing 401, a dead core answers the permanent
// 503 backend_unavailable with exactly one dispatch attempt (no
// retry on a read), and cancellation propagates without retry. The
// deny shape lives in code_divergence_auth_test.go:
// TestCodeDivergenceDenyBranchShapePinned.

// TestCodeDivergenceTerminatedAs_NilCore503 pins that the native
// routes with no core origin configured answer the permanent local
// 503 in both negotiated shapes, with zero upstream contact.
func TestCodeDivergenceTerminatedAs_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)

	calls := []struct {
		name string
		path string
		body string
	}{
		{"POST findings", codeDivergenceFindingsPath, codeDivergenceFindingsBodies[0]},
		{"POST investigate", codeDivergenceInvestigatePath, codeDivergenceInvestigateBodies[0]},
		{"POST report", codeDivergenceReportPath, codeDivergenceReportBodies[0]},
	}
	for _, leg := range calls {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.name + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := codeDivergenceAuthedPost(leg.path, leg.body, accept)
				rec := httptest.NewRecorder()
				srv.serveCodeDivergenceTerminatedAs(rec, req, true)

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

// TestCodeDivergenceRelayExplicit_NilCore503 pins the fail-closed
// relay: an unresolvable credential class with no core origin
// configured answers the permanent local 503 on the relay lane
// (never Retry-After).
func TestCodeDivergenceRelayExplicit_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)

	for _, leg := range []struct {
		path string
		body string
	}{
		{codeDivergenceFindingsPath, codeDivergenceFindingsBodies[0]},
		{codeDivergenceReportPath, codeDivergenceReportBodies[0]},
	} {
		t.Run("POST "+leg.path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, leg.path, strings.NewReader(leg.body))
			req.Header.Set("Authorization", "Bearer eshu_sc_test")
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

// TestCodeDivergenceNative_DeadCoreIs503WithoutRetry pins dead-core
// behavior on the native routes: permanent 503 backend_unavailable
// with no Retry-After, in both negotiated shapes, with the attempt
// count proving exactly one dispatch attempt (no retry on a read).
func TestCodeDivergenceNative_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	calls := []struct {
		name string
		path string
	}{
		{"POST findings", codeDivergenceFindingsPath},
		{"POST investigate", codeDivergenceInvestigatePath},
		{"POST report", codeDivergenceReportPath},
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

				req := authedRequest(http.MethodPost, leg.path)
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

// TestCodeDivergenceNative_CanceledContextFailsClosedWithoutRetry
// pins cancellation propagation on the native routes: a canceled
// caller gets the permanent 503 backend_unavailable with no retry,
// pre-cancel and mid-flight, with the attempt count proving a single
// dispatch attempt in both cases.
func TestCodeDivergenceNative_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cancel string // "before" serves a pre-canceled context; "during" cancels mid-flight
	}{
		{"pre-cancel", "before"},
		{"mid-flight", "during"},
	}
	for _, tc := range cases {
		for _, path := range []string{codeDivergenceFindingsPath, codeDivergenceReportPath} {
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

// TestCodeDivergenceNative_ParallelProbe pins that concurrent
// callers each get one terminated read: 12 parallel calls across the
// three legs answer 200 with the relayed core bytes and land 12
// upstream hits (one forward per caller, no dest coalescing and no
// dest retry).
func TestCodeDivergenceNative_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"findings":[]}`)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	paths := []string{codeDivergenceFindingsPath, codeDivergenceInvestigatePath, codeDivergenceReportPath}
	const callers = 12
	var wg sync.WaitGroup
	failures := make([]string, callers)
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := paths[i%len(paths)]
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
