// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

// Admin input-invalid-facts query bounded-list parity (satellite
// eshu-hq/eshu-api#326). This file owns the local matrix half of the
// lane proof: the v1 alias, the census, the deny branch, the
// fail-closed origin, the spec key, the dest-side benchmark, and the
// gate matrix (denial, foreign relay, junk cookie). The route freeze,
// the local termination, the required-field/bounds parity, the apilib
// matrix, and the zero-core P-READ proof live in
// api/admin_inputinvalid_terminate_test.go.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// TestAdminInputInvalidFactsV1AliasServesLocal pins that the v1 alias
// rewrite lands the promoted route on the same local handler: the
// shared-key v1 call serves the store body with zero core contact.
func TestAdminInputInvalidFactsV1AliasServesLocal(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey).WithInputInvalidStore(&fakeInputInvalidStore{items: []apilibadmin.InputInvalidFact{
		{FactID: "fact-9"},
	}})
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	req := inputInvalidAuthedPOST(t, "/api/v1/admin/input-invalid-facts/query", inputInvalidValidBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"fact-9"`) {
		t.Errorf("body = %s, want the store row", rec.Body.String())
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (v1 alias serves locally)", core.hits)
	}
	counter.AssertZero(t, "v1 alias local termination")
}

// TestAdminInputInvalidFactsCensus is the static-analysis half of the
// zero-core claim: the shared-key handler closure reaches no core
// sink. The foreign classes relay by design (pinned above), so the
// RouteClaims cover only the proven shared-key class (decision 4).
func TestAdminInputInvalidFactsCensus(t *testing.T) {
	t.Parallel()

	AssertNoCorePath(t, []RouteClaim{
		{Pattern: "POST /api/v0/admin/input-invalid-facts/query", Handler: "Server.serveAdminInputInvalidFactsTerminated", Credential: "shared-key"},
	})
}

// TestAdminInputInvalidFactsDenyBranchShapePinned pins the deny shape
// for the day dest gains a scoped/session resolver: a non-AllScopes
// identity is denied with the documented family 403 in both
// negotiated shapes and the core is never contacted. The branch is
// unreachable on the live route today (the wave gate passes true);
// this pins the contract, not a live path.
func TestAdminInputInvalidFactsDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey).WithInputInvalidStore(&fakeInputInvalidStore{})

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := inputInvalidAuthedPOST(t, "/api/v0/admin/input-invalid-facts/query", inputInvalidValidBody)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		srv.serveAdminInputInvalidFactsTerminatedAs(rec, req, false)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Accept %q: status = %d, want 403", accept, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), d3FamilyDenyMessage) {
			t.Errorf("Accept %q: body = %q, want the family-403 message", accept, body)
		}
	}
}

// TestAdminInputInvalidFactsUnconfiguredOriginFailsClosed pins the
// fail-closed origin on the terminated path: without a valid core
// origin every shared-key call answers the permanent local 503 (no
// Retry-After) in both wire shapes, with zero store contact.
func TestAdminInputInvalidFactsUnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"", "://missing-scheme-host"} {
		store := &fakeInputInvalidStore{}
		srv := NewServer(origin, skeletonTestKey).WithInputInvalidStore(store)
		handler := srv.Handler(DefaultV0Sunset)

		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := inputInvalidAuthedPOST(t, "/api/v0/admin/input-invalid-facts/query", inputInvalidValidBody)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("origin %q Accept %q: status = %d, want 503", origin, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("origin %q Accept %q: Retry-After is set on the permanent 503", origin, accept)
			}
		}
		if store.calls != 0 {
			t.Errorf("origin %q: store calls = %d, want 0 (fail closed before the store)", origin, store.calls)
		}
	}
}

// TestAdminInputInvalidFactsSpecKeyServed pins that the promotion
// keeps the route's served-spec entry: keyed callers get the
// dest-assembled document with the input-invalid-facts query path key
// in both negotiated shapes, with or without a core origin
// configured.
func TestAdminInputInvalidFactsSpecKeyServed(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		srv := NewServer(origin, skeletonTestKey).WithInputInvalidStore(&fakeInputInvalidStore{})
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodGet, "/api/v0/openapi.json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("origin %q: status = %d, want 200", origin, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("origin %q: served spec is not valid JSON: %v", origin, err)
		}
		paths, _ := doc["paths"].(map[string]any)
		item, ok := paths["/api/v0/admin/input-invalid-facts/query"].(map[string]any)
		if !ok {
			t.Errorf("origin %q: served spec lacks the input-invalid-facts query key", origin)
			continue
		}
		if _, ok := item["post"].(map[string]any); !ok {
			t.Errorf("origin %q: served spec lacks post under the input-invalid-facts query key", origin)
		}
	}
}

// BenchmarkAdminInputInvalidFactsTerminated measures the promoted
// path for a shared-key query POST through the full production chain.
// The stub core answers from memory but the lane never dials it, and
// the fake store answers from memory too — the numbers bound
// dest-side cost only (bounded pool arithmetic, no core).
func BenchmarkAdminInputInvalidFactsTerminated(b *testing.B) {
	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	srv := NewServer(upstream.URL, skeletonTestKey).WithInputInvalidStore(&fakeInputInvalidStore{})
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	warmReq := authedRequest(http.MethodPost, "/api/v0/admin/input-invalid-facts/query")
	warmReq.Body = io.NopCloser(strings.NewReader(inputInvalidValidBody))
	warmRec := httptest.NewRecorder()
	handler.ServeHTTP(warmRec, warmReq)
	if warmRec.Code != http.StatusOK {
		b.Fatalf("warmup status = %d, want 200", warmRec.Code)
	}
	if got := counter.Calls(); got != 0 {
		b.Fatalf("warmup core-origin calls = %d, want 0 (no core)", got)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := authedRequest(http.MethodPost, "/api/v0/admin/input-invalid-facts/query")
		req.Body = io.NopCloser(strings.NewReader(inputInvalidValidBody))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
	if got := counter.Calls(); got != 0 {
		b.Fatalf("core-origin calls = %d, want 0 (no core)", got)
	}
}

// TestAdminInputInvalidFactsDenial401ZeroCore pins that the promoted
// route keeps the dest 401 shape: credential-less callers get the
// envelope error or the plain error_code body with the bare Bearer
// challenge on both spellings — and the core is never contacted.
func TestAdminInputInvalidFactsDenial401ZeroCore(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey).WithInputInvalidStore(&fakeInputInvalidStore{})
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/admin/input-invalid-facts/query",
		"/api/v0/admin/input-invalid-facts/query/",
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(inputInvalidValidBody))
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("POST %s Accept %q: status = %d, want 401", target, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("POST %s Accept %q: WWW-Authenticate = %q, want bare Bearer", target, accept, got)
			}
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (gate denials never dispatch)", core.hits)
	}
	counter.AssertZero(t, "credential-less denial")
}

// TestAdminInputInvalidFactsForeignRelayExplicit pins the never-invent
// rule for the promoted route: a call carrying a credential class
// dest cannot resolve relays explicitly to the core on the relay
// lane — core judges, core bytes come back untouched — and never
// answers a dest-invented failure.
func TestAdminInputInvalidFactsForeignRelayExplicit(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel).WithInputInvalidStore(&fakeInputInvalidStore{})
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, "/api/v0/admin/input-invalid-facts/query", strings.NewReader(inputInvalidValidBody))
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Errorf("%s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, core.hits)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("%s: status = %d, want the relayed core 200", tc.name, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != core.body {
				t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, body, core.body)
			}
			if got := tel.snapshot().dispatches["admin"][viaRelay][outcomeCoreOK]; got != 1 {
				t.Errorf("%s: family admin relay core_ok = %d, want 1", tc.name, got)
			}
		})
	}
}

// TestAdminInputInvalidFactsJunkCookieStays401 pins the D2 follow-up
// cookie lesson for the promoted route: only session-cookie-shaped
// credentials relay. Any other Cookie stays dest-401 with zero core
// contact, exactly like a credential-less caller, in both negotiated
// shapes.
func TestAdminInputInvalidFactsJunkCookieStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey).WithInputInvalidStore(&fakeInputInvalidStore{})
	handler := srv.Handler(DefaultV0Sunset)

	for _, cookie := range []string{
		"theme=dark",
		"unrelated=ignore; other=1",
		"eshu_session_stale=",
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodPost, "/api/v0/admin/input-invalid-facts/query", strings.NewReader(inputInvalidValidBody))
			req.Header.Set("Accept", accept)
			req.Header.Set("Cookie", cookie)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("cookie %q Accept %q: status = %d, want 401 (junk cookies never relay)", cookie, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("cookie %q Accept %q: WWW-Authenticate = %q, want bare Bearer", cookie, accept, got)
			}
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (junk cookies never reach the core)", core.hits)
	}
}
