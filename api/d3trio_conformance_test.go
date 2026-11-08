// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

// D3 Gap 02 trio conformance (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#70). TDD record: this suite is RED as committed —
// the frozen wave below mirrors the three Gap 02 pins
// (DeltaUnionCompletenessReadAdmission v0, ReindexProgressReadAdmission
// v0, SharedProjectionTuningReport v0 at the core merge 4a587431fd),
// and the behavior tests require every trio route to terminate
// natively for shared-key callers while relaying unresolvable
// identity classes explicitly. Until the lane registers the routes,
// every trio call crosses the generic byte-relay (or the key gate),
// so the ahead-of-relay assertions fail first. That red is the
// recorded expectation, not a defect.
//
// Cookie rule (D2 follow-up lesson): only session-cookie-shaped
// credentials relay — a Cookie header carrying eshu_session or
// __Host-eshu_session relays explicitly so core judges the browser
// session; any other Cookie (junk) stays dest-401 with zero core
// contact, exactly like a credential-less caller.

// TestD3TrioWaveFrozen pins the exact Gap 02 termination surface. A
// wave change (route added, removed, renamed, or rehomed) fails here
// until the eshu spec, the skeleton table, and this mirror move
// together — the same frozen-set pattern as TestD3WaveFrozen.
func TestD3TrioWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/admin/delta-union/completeness admin gated paths/status/delta_union_completeness.go",
		"GET /api/v0/admin/reindex/progress admin gated paths/status/reindex_progress.go",
		"GET /api/v0/admin/shared-projection/tuning-report admin gated paths/status/admin.go",
	}
	if len(d3TrioWave) != len(want) {
		t.Fatalf("d3TrioWave holds %d routes, want frozen %d: move the spec, the skeleton table, and this mirror together",
			len(d3TrioWave), len(want))
	}
	got := make([]string, 0, len(d3TrioWave))
	for _, route := range d3TrioWave {
		got = append(got, route.Method+" "+route.Path+" "+route.Family+" "+route.Serve+" "+route.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("d3TrioWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// d3TrioConcretePaths fills the wave's query strings so the behavior
// tests drive real mux matches. The delta-union read requires at
// least one scope filter and the reindex progress read at least one
// scope_id; the tuning report takes none. Trailing-slash spellings
// ride the same native registration (never-split).
var d3TrioConcretePaths = map[string]string{
	"GET /api/v0/admin/delta-union/completeness":        "/api/v0/admin/delta-union/completeness?scope=s1",
	"GET /api/v0/admin/reindex/progress":                "/api/v0/admin/reindex/progress?scope_id=s1",
	"GET /api/v0/admin/shared-projection/tuning-report": "/api/v0/admin/shared-projection/tuning-report",
}

// TestD3TrioWaveRoutesAheadOfRelay pins that every trio route is an
// explicit registration ahead of the generic proxy: a shared-key call
// must file its dispatch on the native lane (viaNative), not the relay
// lane. Until the lane registers the routes, every call crosses the
// generic relay and this test is RED.
func TestD3TrioWaveRoutesAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, entry := range d3TrioWave {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			key := entry.Method + " " + entry.Path
			target, ok := d3TrioConcretePaths[key]
			if !ok {
				t.Fatalf("no concrete path for trio route %q", key)
			}
			req := authedRequest(entry.Method, target)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(target)
			if family != entry.Family {
				t.Fatalf("classifyFamily(%q) = %q, want trio family %q", target, family, entry.Family)
			}
			lanes, ok := snap.dispatches[family]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit trio route ahead of relay", family)
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key trio call", family, lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", family, got)
			}
		})
	}
}

// TestD3TrioWaveRelayExplicitForUnresolvableIdentities pins the
// D-wave ruling's never-invent rule for the trio: a wave-gated route
// carrying a credential class dest cannot resolve (a scoped token, an
// OIDC bearer, a browser session cookie) must relay explicitly to the
// core — the core judges, core bytes come back untouched — and must
// never answer a dest-invented 401, 403, 404, or 500. The lane files
// on viaRelay so a reader can tell core-served from dest-terminated.
// Until the lane registers the routes, the key gate 401s these
// callers with zero core contact, so this test is RED.
func TestD3TrioWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, entry := range d3TrioWave {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()

			for _, tc := range []struct {
				name   string
				header map[string]string
			}{
				{"scoped bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
				{"oidc bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
				{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
				{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
			} {
				core := &stubCore{
					status: http.StatusOK,
					body:   `{"data":[],"error":null,"truth":null}`,
				}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				key := entry.Method + " " + entry.Path
				req := httptest.NewRequest(entry.Method, d3TrioConcretePaths[key], nil)
				for k, v := range tc.header {
					req.Header.Set(k, v)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Errorf("%s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, core.hits)
				}
				if rec.Code != http.StatusOK {
					t.Errorf("%s: status = %d, want the relayed core 200 (never 401/403/404/500-invented)", tc.name, rec.Code)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != core.body {
					t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, body, core.body)
				}
				family := classifyFamily(d3TrioConcretePaths[key])
				if got := tel.snapshot().dispatches[family][viaRelay][outcomeCoreOK]; got != 1 {
					t.Errorf("%s: family %q relay core_ok = %d, want 1", tc.name, family, got)
				}
			}
		})
	}
}

// TestD3TrioWaveJunkCookieStays401 pins the D2 follow-up cookie
// lesson for the trio: only session-cookie-shaped credentials relay.
// A Cookie header with no eshu_session (or __Host-eshu_session)
// member is not a browser session — it stays dest-401 with zero core
// contact, exactly like a credential-less caller, in both negotiated
// shapes.
func TestD3TrioWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, entry := range d3TrioWave {
		for _, cookie := range []string{
			"theme=dark",
			"unrelated=ignore; other=1",
			"eshu_session_stale=",
		} {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				key := entry.Method + " " + entry.Path
				req := httptest.NewRequest(entry.Method, d3TrioConcretePaths[key], nil)
				req.Header.Set("Accept", accept)
				req.Header.Set("Cookie", cookie)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusUnauthorized {
					t.Errorf("%s %s cookie %q Accept %q: status = %d, want 401 (junk cookies never relay)",
						entry.Method, entry.Path, cookie, accept, rec.Code)
				}
				if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
					t.Errorf("%s %s cookie %q Accept %q: WWW-Authenticate = %q, want bare Bearer",
						entry.Method, entry.Path, cookie, accept, got)
				}
			}
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (junk cookies never reach the core)", core.hits)
	}
}

// TestD3TrioGated_TrailingSlashStaysNative pins the never-split rule
// for the trio: the trailing-slash spelling of each trio route rides
// the wave gate (shared-key terminates on the native lane), not the
// generic relay.
func TestD3TrioGated_TrailingSlashStaysNative(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	targets := []struct{ target, wantPath, wantQuery string }{
		{"/api/v0/admin/delta-union/completeness/?scope=s1", "/api/v0/admin/delta-union/completeness/", "scope=s1"},
		{"/api/v0/admin/reindex/progress/?scope_id=s1", "/api/v0/admin/reindex/progress/", "scope_id=s1"},
		{"/api/v0/admin/shared-projection/tuning-report/", "/api/v0/admin/shared-projection/tuning-report/", ""},
	}
	for _, tc := range targets {
		req := authedRequest(http.MethodGet, tc.target)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", tc.target, rec.Code)
		}
		if core.lastPath != tc.wantPath {
			t.Errorf("GET %s: upstream path = %q, want the slash spelling forwarded", tc.target, core.lastPath)
		}
		if core.lastQuery != tc.wantQuery {
			t.Errorf("GET %s: upstream query = %q, want %q", tc.target, core.lastQuery, tc.wantQuery)
		}
	}
	snap := tel.snapshot()
	if got := snap.dispatches["admin"][viaNative][outcomeCoreOK]; got < 3 {
		t.Errorf("family admin native core_ok = %d, want >= 3 (slash spellings on the native lane)", got)
	}
}

// TestD3TrioGated_V1AliasTerminates pins that the v1 alias rewrite
// lands trio routes on the same native handler: the core sees the
// rewritten v0 path.
func TestD3TrioGated_V1AliasTerminates(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range []struct{ v1, v0 string }{
		{"/api/v1/admin/delta-union/completeness?scope=s1", "/api/v0/admin/delta-union/completeness"},
		{"/api/v1/admin/reindex/progress?scope_id=s1", "/api/v0/admin/reindex/progress"},
		{"/api/v1/admin/shared-projection/tuning-report", "/api/v0/admin/shared-projection/tuning-report"},
	} {
		req := authedRequest(http.MethodGet, tc.v1)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200", tc.v1, rec.Code)
		}
		if core.lastPath != tc.v0 {
			t.Errorf("GET %s: upstream path = %q, want the rewritten %s", tc.v1, core.lastPath, tc.v0)
		}
	}
}
