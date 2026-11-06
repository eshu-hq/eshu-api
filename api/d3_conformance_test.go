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

// D3 admin/identity/recovery reads conformance (Refs eshu-hq/eshu#9669,
// part of #6707). TDD record: this suite is RED as committed — the
// frozen wave below mirrors the D1-pinned read families (Admin,
// AdminIdentity, GovernanceAudit, TerminalFailure read admissions at
// the satellite pin #50) minus the families D2 already terminated
// (provider-config reads, admin sign-in-policy), and the behavior
// tests require every wave route to terminate natively for shared-key
// callers while relaying unresolvable identity classes explicitly.
// Until the lane lands, every wave call crosses the generic byte-relay
// (or the key gate), so the ahead-of-relay assertions fail first. That
// red is the recorded expectation, not a defect.
//
// Scope (16 routes, from go/cmd/api/CUTOVER_OWNERSHIP.md at the D2
// lane base plus the four D1 pins; no guessing):
//
//   - AdminReadAdmission v0 (family admin): the 6 query POSTs served
//     through ReadStore (work items, dead letters, input-invalid
//     facts, poisoned links, replay events, decisions + decision
//     evidence). The 3 Admin GETs (delta-union completeness,
//     reindex progress, shared-projection tuning-report) stay relay:
//     their readers (DeltaUnionCompletenessChecker,
//     ReindexProgressReader, the static tuning report) carry no
//     versioned pin, and the minting rule admits no blanket pins.
//   - AdminIdentityReadAdmission v0 (family admin_identity): the 6
//     tenant-scoped identity GETs (api-tokens, idp-group-mappings,
//     idp-providers, role-assignments, roles, local invitations).
//   - GovernanceAuditReadAdmission v0 (family governance_audit): the 2
//     tenant-admin audit GETs (events, summary).
//   - TerminalFailureReadAdmission v0 (family terminal_failure): the 2
//     collector-failures query POSTs (acknowledgements list, failure
//     discovery list).
//
// Cookie rule (D2 follow-up lesson): only session-cookie-shaped
// credentials relay — a Cookie header carrying eshu_session or
// __Host-eshu_session relays explicitly so core judges the browser
// session; any other Cookie (junk) stays dest-401 with zero core
// contact, exactly like a credential-less caller.

// TestD3WaveFrozen pins the exact D3 termination surface. A wave
// change (route added, removed, renamed, or rehomed) fails here until
// the eshu spec, the skeleton table, and this mirror move together —
// the same frozen-set pattern as TestLeafBWaveFrozen.
func TestD3WaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/admin/work-items/query admin gated paths/status/admin.go",
		"POST /api/v0/admin/decisions/query admin gated paths/status/admin.go",
		"POST /api/v0/admin/dead-letters/query admin gated paths/status/admin.go",
		"POST /api/v0/admin/input-invalid-facts/query admin gated paths/status/admin.go",
		"POST /api/v0/admin/changed-since/poisoned-links/query admin gated paths/status/admin.go",
		"POST /api/v0/admin/replay-events/query admin gated paths/status/admin.go",
		"GET /api/v0/auth/admin/api-tokens admin_identity gated paths/auth/admin_reads.go",
		"GET /api/v0/auth/admin/idp-group-mappings admin_identity gated paths/auth/admin_reads.go",
		"GET /api/v0/auth/admin/idp-providers admin_identity gated paths/auth/admin_reads.go",
		"GET /api/v0/auth/admin/role-assignments admin_identity gated paths/auth/admin_reads.go",
		"GET /api/v0/auth/admin/roles admin_identity gated paths/auth/admin_reads.go",
		"GET /api/v0/auth/local/invitations admin_identity gated paths/auth/routes.go",
		"GET /api/v0/auth/admin/audit/events governance_audit gated paths/auth/admin_reads.go",
		"GET /api/v0/auth/admin/audit/summary governance_audit gated paths/auth/admin_reads.go",
		"POST /api/v0/admin/collector-failures/acknowledgements/query terminal_failure gated paths/status/collector_terminal_failure_acknowledgement.go",
		"POST /api/v0/admin/collector-failures/query terminal_failure gated paths/status/collector_terminal_failure_acknowledgement.go",
	}
	if len(d3Wave) != len(want) {
		t.Fatalf("d3Wave holds %d routes, want frozen %d: move the spec, the skeleton table, and this mirror together",
			len(d3Wave), len(want))
	}
	got := make([]string, 0, len(d3Wave))
	for _, route := range d3Wave {
		got = append(got, route.Method+" "+route.Path+" "+route.Family+" "+route.Serve+" "+route.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("d3Wave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// d3ConcretePaths fills the wave's path wildcards so the behavior
// tests drive real mux matches. Trailing-slash spellings ride the same
// native registration (never-split), so each entry also names its
// slash variant where the mux distinguishes it.
var d3ConcretePaths = map[string]string{
	"POST /api/v0/admin/work-items/query":                          "/api/v0/admin/work-items/query",
	"POST /api/v0/admin/decisions/query":                           "/api/v0/admin/decisions/query",
	"POST /api/v0/admin/dead-letters/query":                        "/api/v0/admin/dead-letters/query",
	"POST /api/v0/admin/input-invalid-facts/query":                 "/api/v0/admin/input-invalid-facts/query",
	"POST /api/v0/admin/changed-since/poisoned-links/query":        "/api/v0/admin/changed-since/poisoned-links/query",
	"POST /api/v0/admin/replay-events/query":                       "/api/v0/admin/replay-events/query",
	"GET /api/v0/auth/admin/api-tokens":                            "/api/v0/auth/admin/api-tokens",
	"GET /api/v0/auth/admin/idp-group-mappings":                    "/api/v0/auth/admin/idp-group-mappings",
	"GET /api/v0/auth/admin/idp-providers":                         "/api/v0/auth/admin/idp-providers",
	"GET /api/v0/auth/admin/role-assignments":                      "/api/v0/auth/admin/role-assignments?user_id=u1",
	"GET /api/v0/auth/admin/roles":                                 "/api/v0/auth/admin/roles",
	"GET /api/v0/auth/local/invitations":                           "/api/v0/auth/local/invitations",
	"GET /api/v0/auth/admin/audit/events":                          "/api/v0/auth/admin/audit/events?limit=5",
	"GET /api/v0/auth/admin/audit/summary":                         "/api/v0/auth/admin/audit/summary",
	"POST /api/v0/admin/collector-failures/acknowledgements/query": "/api/v0/admin/collector-failures/acknowledgements/query",
	"POST /api/v0/admin/collector-failures/query":                  "/api/v0/admin/collector-failures/query",
}

// TestD3WaveRoutesAheadOfRelay pins that every wave route is an
// explicit registration ahead of the generic proxy: a shared-key call
// must file its dispatch on the native lane (viaNative), not the relay
// lane. Today the wave registers nothing, so every call crosses the
// generic relay and this test is RED.
func TestD3WaveRoutesAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, entry := range d3Wave {
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
			target, ok := d3ConcretePaths[key]
			if !ok {
				t.Fatalf("no concrete path for wave route %q", key)
			}
			req := authedRequest(entry.Method, target)
			if entry.Method == http.MethodPost {
				req.Body = io.NopCloser(http.NoBody)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			family := classifyFamily(target)
			if family != entry.Family {
				t.Fatalf("classifyFamily(%q) = %q, want wave family %q", target, family, entry.Family)
			}
			lanes, ok := snap.dispatches[family]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", family)
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", family, lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", family, got)
			}
		})
	}
}

// TestD3WaveRelayExplicitForUnresolvableIdentities pins the D-wave
// ruling's never-invent rule: a wave-gated route carrying a credential
// class dest cannot resolve (a scoped token, an OIDC bearer, a browser
// session cookie) must relay explicitly to the core — the core judges,
// core bytes come back untouched — and must never answer a
// dest-invented 401, 403, 404, or 500. The lane files on viaRelay so a
// reader can tell core-served from dest-terminated. Today the key gate
// 401s these callers with zero core contact, so this test is RED.
func TestD3WaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, entry := range d3Wave {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()

			for _, tc := range []struct {
				name   string
				header map[string]string
			}{
				{"scoped bearer", map[string]string{"Authorization": "Bearer scoped-token"}},
				{"oidc bearer", map[string]string{"Authorization": "Bearer oidc-id-token"}},
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
				req := httptest.NewRequest(entry.Method, d3ConcretePaths[key], nil)
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
				family := classifyFamily(d3ConcretePaths[key])
				if got := tel.snapshot().dispatches[family][viaRelay][outcomeCoreOK]; got != 1 {
					t.Errorf("%s: family %q relay core_ok = %d, want 1", tc.name, family, got)
				}
			}
		})
	}
}

// TestD3WaveJunkCookieStays401 pins the D2 follow-up cookie lesson:
// only session-cookie-shaped credentials relay. A Cookie header with
// no eshu_session (or __Host-eshu_session) member is not a browser
// session — it stays dest-401 with zero core contact, exactly like a
// credential-less caller, in both negotiated shapes.
func TestD3WaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, entry := range d3Wave {
		for _, cookie := range []string{
			"theme=dark",
			"unrelated=ignore; other=1",
			"eshu_session_stale=",
		} {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				key := entry.Method + " " + entry.Path
				req := httptest.NewRequest(entry.Method, d3ConcretePaths[key], nil)
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
