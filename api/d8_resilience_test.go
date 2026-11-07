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

// D8 session-resolver tier-C clone resilience proof (Refs
// eshu-hq/eshu#9669, part of #6707). The clone holds the same failure
// contract as the D7 lane: fail closed with no core configured, exactly
// one upstream attempt on every failure shape, and no dest retry on
// cancellation or an unstable core. Transport 500s/503s are retryable
// only end-to-end by the client, never by dest.

// d8Spellings returns every native spelling: each exact route plus its
// trailing-slash twin — except the invitations-create gated route (a
// d8SlashSkip entry: its slash subtree would panic against the
// invitations/{invite_id}/revoke wildcard sibling) and every public
// carve-out route (exact paths only: a slash twin would be a subtree
// swallowing /login/* past the auth bypass). Those spellings stay on
// the generic relay by design, pinned in the dispatch suite.
func d8Spellings() []struct {
	method string
	path   string
} {
	var out []struct {
		method string
		path   string
	}
	push := func(method, path string) {
		out = append(out, struct {
			method string
			path   string
		}{method, path})
	}
	for _, route := range d8GatedRoutes {
		method, path, _ := strings.Cut(route, " ")
		push(method, path)
		if !d8SlashSkip[route] {
			push(method, path+"/")
		}
	}
	for _, route := range d8PublicRoutes {
		method, path, _ := strings.Cut(route, " ")
		push(method, path)
	}
	return out
}

// TestD8Native_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured every native spelling answers
// the permanent 503 backend_unavailable instead of panicking, in both
// negotiated shapes.
func TestD8Native_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range d8Spellings() {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			var req *http.Request
			if d8IsPublicPath(spelling.path) {
				req = httptest.NewRequest(spelling.method, spelling.path, nil)
			} else {
				req = authedRequest(spelling.method, spelling.path)
			}
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

// d8IsPublicPath reports whether path is one of the four pre-auth
// public carve-out paths (served keyless).
func d8IsPublicPath(path string) bool {
	for _, route := range d8PublicRoutes {
		if _, p, _ := strings.Cut(route, " "); p == path {
			return true
		}
	}
	return false
}

// TestD8Native_BothSpellingsWriteThrough pins that both spellings of
// every twinned D8 path stay on the native route: method and path (as
// spelled) forward to the same core route.
func TestD8Native_BothSpellingsWriteThrough(t *testing.T) {
	t.Parallel()

	for _, spelling := range d8Spellings() {
		core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		var req *http.Request
		if d8IsPublicPath(spelling.path) {
			req = httptest.NewRequest(spelling.method, spelling.path, nil)
		} else {
			req = authedRequest(spelling.method, spelling.path)
		}
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

// TestD8Native_DeadCoreIs503WithoutRetry pins the dead-core shape on
// the native route: permanent 503 backend_unavailable with no
// Retry-After, in both negotiated shapes, with the attempt count proving
// exactly one dispatch attempt (no retry on a write). The unstable-core
// half of the tier-C proof box: a 500 or a retryable 503 from the core
// answers the same way — one forward, core bytes relayed — and only the
// client may retry end-to-end (covered in the write-through matrix,
// which pins exactly one upstream hit on those rows).
func TestD8Native_DeadCoreIs503WithoutRetry(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, "/api/v0/auth/local/api-tokens")
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

// TestD8Native_CanceledContextFailsClosedWithoutRetry pins
// cancellation propagation on the native route: a canceled caller gets
// the permanent 503 backend_unavailable with no retry, pre-cancel and
// mid-flight, with the attempt count proving a single dispatch attempt
// in both cases.
func TestD8Native_CanceledContextFailsClosedWithoutRetry(t *testing.T) {
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
			req := authedRequest(http.MethodPost, "/api/v0/auth/local/mfa/totp/confirm").WithContext(ctx)
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

// TestD8Native_ParallelProbe pins that concurrent callers each get one
// write-through: 16 parallel token-create POSTs answer 200 with the
// relayed core bytes and land 16 upstream hits (one forward per caller,
// no dest coalescing and no dest retry). Dest holds no ledger, so it
// serializes nothing — which is exactly the no-worse-than-core proof:
// the same burst against the core directly converges the same way.
func TestD8Native_ParallelProbe(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"created"}`)
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
			req := authedRequest(http.MethodPost, "/api/v0/auth/local/api-tokens")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				failures[i] = fmt.Sprintf("caller %d: status = %d, want 200", i, rec.Code)
				return
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), `"status":"created"`) {
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

// TestD8Native_V1AliasOrderingUnchanged pins that the v1 alias rewrite
// lands D8 POSTs on their native handlers: the bootstrap POST reaches
// the gated lane and the login POST reaches the public carve-out, both
// with the rewritten /api/v0/* path and the v1 deprecation headers
// absent.
func TestD8Native_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	gated := authedRequest(http.MethodPost, "/api/v1/auth/local/bootstrap")
	gated.Body = io.NopCloser(strings.NewReader(`{"username":"owner"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, gated)

	if core.hits != 1 || core.lastPath != "/api/v0/auth/local/bootstrap" {
		t.Errorf("gated: hits = %d path = %q, want alias rewrite to the native v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("gated: Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("gated: status = %d, want relayed 200", rec.Code)
	}

	public := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", strings.NewReader(`{}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, public)

	if core.hits != 2 || core.lastPath != "/api/v0/auth/local/login" {
		t.Errorf("public: hits = %d path = %q, want alias rewrite to the carve-out v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("public: Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("public: status = %d, want relayed 200", rec.Code)
	}
}

// TestD8Native_NeighborLanesStillNative pins that the D7 and setup
// lanes are unaffected by the D8 registrations: an admin-four POST
// still rides its D7 native route and a setup claim POST still rides
// the public setup carve-out, each with exactly one upstream hit. The
// D8 auth paths share no registration key with those lanes beyond
// /api/v0/auth, and Go matches method and path together, so the lanes
// never merge.
func TestD8Native_NeighborLanesStillNative(t *testing.T) {
	t.Parallel()

	for _, target := range []string{
		"POST /api/v0/admin/skip",
		"POST /api/v0/auth/setup/claim",
		"POST /api/v0/auth/local/invitations/invite-1/revoke",
	} {
		method, path, _ := strings.Cut(target, " ")
		core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		var req *http.Request
		switch path {
		case "/api/v0/auth/setup/claim":
			req = httptest.NewRequest(method, path, nil)
		case "/api/v0/auth/local/invitations/invite-1/revoke":
			req = authedRequest(method, path)
		default:
			req = authedRequest(method, path)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != path || core.lastMethod != method {
			t.Errorf("path %q: hits = %d method = %q lastPath = %q, want the neighboring native lane to forward once untouched",
				path, core.hits, core.lastMethod, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("path %q: status = %d, want 200", path, rec.Code)
		}
	}
}
