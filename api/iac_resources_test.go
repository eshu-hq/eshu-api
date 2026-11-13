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

// W3 IaC-6 resources leg (Refs eshu-hq/eshu-api#102, part of
// eshu-hq/eshu#9669): GET /api/v0/iac/resources terminates ahead of the
// relay on the explicit native route, cleared on
// IaCInventorySearchAdmission v0 (eshu-hq/eshu#10578). POST
// /api/v0/iac/dead stays on the generic relay until the dual-pin dead
// disposition lands; the remaining IaC 5 ride W8a.

// iacResourcesTarget is the candidate-page spelling the parity proof
// exercises, including the keyset query the route must forward intact.
const iacResourcesTarget = "/api/v0/iac/resources?limit=2"

// TestIaCResources_AheadOfRelay pins that the resources leg is an
// explicit registration ahead of the generic proxy: a shared-key GET
// must file its dispatch on the native lane (viaNative), not the relay
// lane, with method, path, query, credential, and correlation
// forwarded intact. This test is the swap-mutant detector for
// dispatch: if the registration is swapped back to the generic
// dispatch, the call files relay and this test goes RED. Until the
// lane lands, the call crosses the generic byte-relay and this test
// is RED.
func TestIaCResources_AheadOfRelay(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":{"total":0},"error":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, iacResourcesTarget)
	req.Header.Set("Accept", EnvelopeMIME)
	req.Header.Set("X-Correlation-ID", "iac-resources-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
	}
	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1 (no retry on terminated reads)", core.hits)
	}
	if core.lastMethod != http.MethodGet {
		t.Errorf("upstream method = %q, want GET", core.lastMethod)
	}
	if core.lastPath != "/api/v0/iac/resources" {
		t.Errorf("upstream path = %q, want the core route", core.lastPath)
	}
	if core.lastQuery != "limit=2" {
		t.Errorf("upstream query = %q, want the candidate-page query forwarded intact", core.lastQuery)
	}
	if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
		t.Errorf("upstream Authorization = %q, want the forwarded Bearer key", core.lastAuth)
	}
	snap := tel.snapshot()
	lanes, ok := snap.dispatches["iac"]
	if !ok {
		t.Fatalf("family %q has no dispatch rows, want the explicit resources route ahead of relay", "iac")
	}
	if lanes[viaNative] == nil {
		t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key resources call", "iac", lanes)
	}
	if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
		t.Errorf("family %q native core_ok = %d, want 1", "iac", got)
	}
}

// TestIaCResources_UnauthenticatedIs401 pins that both resources
// spellings keep the dest 401 shape — and the core is never reached.
func TestIaCResources_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range []string{iacRoutePath(iacResourcesRoute), iacRoutePath(iacResourcesRoute) + "/"} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodGet, spelling, nil)
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%q Accept %q: status = %d, want 401", spelling, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("%q Accept %q: WWW-Authenticate = %q, want bare Bearer", spelling, accept, got)
			}
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (gate denies before dispatch)", core.hits)
	}
}

// TestIaCResources_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured both resources spellings
// answer the permanent 503 backend_unavailable instead of panicking,
// in both negotiated shapes.
func TestIaCResources_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range []string{iacRoutePath(iacResourcesRoute), iacRoutePath(iacResourcesRoute) + "/"} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := authedRequest(http.MethodGet, spelling)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%q Accept %q: status = %d, want 503", spelling, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%q Accept %q: Retry-After = %q, want none (local 503 is permanent)", spelling, accept, got)
			}
		}
	}
}

// TestIaCResources_RelaysByteForByte pins the termination parity: the
// core's status, headers, and body cross the native resources route
// unmodified — the 200 candidate page, the scoped-grant 403 (dest adds
// no gate; the core judges), the fixed 500 message in its exact core
// bytes, the retryable 503 keeping the core's Retry-After, and the
// keyset paging edge — with exactly one forward attempt per caller.
func TestIaCResources_RelaysByteForByte(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		header http.Header
		body   string
		query  string
	}{
		{"truth page", http.StatusOK, nil, "{\"data\":{\"total\":2,\"truncated\":false},\"error\":null}\n", "limit=2"},
		{
			"grant denial relays untouched",
			http.StatusForbidden, nil,
			"{\"detail\":\"all-scopes authorization is required\",\"error\":\"Forbidden\"}\n",
			"limit=50",
		},
		{
			"search fixed 500",
			http.StatusInternalServerError, nil,
			"{\"detail\":\"IaC resource inventory search failed\",\"error\":\"Internal Server Error\"}\n",
			"limit=50",
		},
		{
			"retryable 503 keeps Retry-After",
			http.StatusServiceUnavailable,
			http.Header{"Retry-After": []string{"5"}},
			"{\"detail\":\"IaC inventory database is unavailable\",\"error\":\"Service Unavailable\"}\n",
			"limit=50",
		},
		{
			"keyset page relays cursor",
			http.StatusOK, nil,
			"{\"data\":{\"total\":201,\"next_cursor\":\"aws_s3_bucket.logs|content-entity:e_9\",\"truncated\":true},\"error\":null}\n",
			"limit=11",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				target := iacRoutePath(iacResourcesRoute) + "?" + tc.query
				req := authedRequest(http.MethodGet, target)
				req.Header.Set("Accept", accept)
				req.Header.Set("X-Correlation-ID", "iac-resources-parity")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Fatalf("%s Accept %q: upstream hits = %d, want exactly 1 (no retry on write-through)", tc.name, accept, core.hits)
				}
				if core.lastMethod != http.MethodGet {
					t.Errorf("%s Accept %q: upstream method = %q, want GET", tc.name, accept, core.lastMethod)
				}
				if core.lastPath != iacRoutePath(iacResourcesRoute) {
					t.Errorf("%s Accept %q: upstream path = %q, want the core route %q", tc.name, accept, core.lastPath, iacRoutePath(iacResourcesRoute))
				}
				if core.lastQuery != tc.query {
					t.Errorf("%s Accept %q: upstream query = %q, want %q", tc.name, accept, core.lastQuery, tc.query)
				}
				if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
					t.Errorf("%s Accept %q: upstream Authorization = %q, want the forwarded Bearer key", tc.name, accept, core.lastAuth)
				}
				if rec.Code != tc.status {
					t.Errorf("%s Accept %q: status = %d, want relayed %d", tc.name, accept, rec.Code, tc.status)
				}
				if tc.header != nil {
					if got := rec.Header().Get("Retry-After"); got != tc.header.Get("Retry-After") {
						t.Errorf("%s Accept %q: Retry-After = %q, want the core's value relayed", tc.name, accept, got)
					}
				}
				if got := rec.Header().Get("X-Correlation-ID"); got != "iac-resources-parity" {
					t.Errorf("%s Accept %q: X-Correlation-ID = %q, want the caller value echoed", tc.name, accept, got)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != tc.body {
					t.Errorf("%s Accept %q: body = %q, want the core bytes %q", tc.name, accept, body, tc.body)
				}
			}
		})
	}
}

// TestIaCResources_TrailingSlashStaysNative pins the never-split rule:
// the trailing-slash spelling forwards once to the same core route
// with the spelled path, so the family is never split across dispatch
// paths.
func TestIaCResources_TrailingSlashStaysNative(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	spelling := iacRoutePath(iacResourcesRoute) + "/"
	req := authedRequest(http.MethodGet, spelling)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 || core.lastPath != spelling {
		t.Errorf("hits = %d lastPath = %q, want the native route to forward as spelled", core.hits, core.lastPath)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// TestIaCResources_MatchesGenericRelay pins the before/after parity at
// the dest layer — the revert-to-relay proof: the same stub-core answer
// crosses the native resources route and the generic relay (POST
// /api/v0/iac/dead, the partial leg that stays on the relay) with
// byte-identical status and body. Removing the native registration
// restores relay bytes exactly, so rollback moves no truth.
func TestIaCResources_MatchesGenericRelay(t *testing.T) {
	t.Parallel()

	const relayPath = "/api/v0/iac/dead"
	bodies := []struct {
		name   string
		status int
		body   string
	}{
		{"truth page", http.StatusOK, "{\"data\":{\"total\":1},\"error\":null}\n"},
		{"fixed 500", http.StatusInternalServerError, "{\"detail\":\"dead IaC scan failed\",\"error\":\"Internal Server Error\"}\n"},
	}
	for _, fixture := range bodies {
		core := &stubCore{status: fixture.status, body: fixture.body}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		serve := func(method, target, payload string) (int, string) {
			req := authedRequest(method, target)
			if payload != "" {
				req.Body = io.NopCloser(strings.NewReader(payload))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			body, _ := io.ReadAll(rec.Result().Body)
			return rec.Code, string(body)
		}
		nativeCode, nativeBody := serve(http.MethodGet, iacResourcesTarget, "")
		relayCode, relayBody := serve(http.MethodPost, relayPath, `{"limit":50}`)

		if nativeCode != relayCode || nativeBody != relayBody {
			t.Errorf("%s: native = (%d, %q), relay = (%d, %q): want byte-identical", fixture.name, nativeCode, nativeBody, relayCode, relayBody)
		}
	}
}

// TestIaCResources_DeadCoreIs503WithoutRetry pins the dead-core shape
// on the native resources route: permanent 503 backend_unavailable
// with no Retry-After, in both negotiated shapes, with the attempt
// count proving exactly one dispatch attempt (no retry on a read).
func TestIaCResources_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodGet, iacResourcesTarget)
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

// TestIaCResources_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native resources route: a canceled
// caller gets the permanent 503 backend_unavailable with no retry,
// pre-cancel and mid-flight, with the attempt count proving a single
// dispatch attempt in both cases.
func TestIaCResources_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
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
			req := authedRequest(http.MethodGet, iacResourcesTarget).WithContext(ctx)
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

// TestIaCResources_ParallelProbe pins that concurrent callers each get
// one write-through: 8 parallel GETs on the resources leg answer 200
// with the relayed core bytes and land 8 upstream hits (one forward
// per caller, no dest coalescing and no dest retry).
func TestIaCResources_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"data":{"total":0},"error":null}`)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	const callers = 8
	var wg sync.WaitGroup
	failures := make([]string, callers)
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := authedRequest(http.MethodGet, iacResourcesTarget)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = iacResourcesTarget
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
