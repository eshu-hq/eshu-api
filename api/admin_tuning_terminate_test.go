// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// Admin shared-projection tuning-report native termination (satellite
// eshu-hq/eshu-api#171, part of the D3 Gap 02 trio promotion). TDD
// record: this suite failed RED before the lane landed (the route
// constant, the terminated handler, and the local_ok outcome did not
// exist, so every assertion below failed at compile; the shared-key
// legs rode the generic D3 trio gate, so the zero-core assertions
// failed) and is green with it.
//
// Lane rule (arbiter D-wave ruling, decision 4, via the D3 wave): the
// shared-key class terminates locally on the pinned apilib static
// body with zero core contact and zero store contact; credential
// classes dest cannot resolve (scoped tokens, OIDC bearers, browser
// sessions) relay explicitly on the untouched D3 gate so core judges
// them; credential-less and junk-cookie callers keep the standing 401
// with zero core contact. The only dest-synthesized errors on the
// terminated path are the gate 401 and the permanent local 503 when
// no core origin is configured (never Retry-After).
//
// Proof vocabulary: the no-store static leaf emits no store spans, so
// silence is the span evidence and P-FLOW (not P-READ) carries the
// RunProof case — P-READ success requires a read span the static body
// never emits. The flow proves zero core-origin calls with the
// counting transport across the success and denial legs alike.

// adminTuningPinnedBody is the exact frozen report body
// (SharedProjectionTuningReport v0: status not_applicable plus the
// frozen detail text, trailing encoder newline included). It must stay
// byte-identical to the apilib WriteTuningReport output the handler
// serves and to the core constants the pin mirrors.
const adminTuningPinnedBody = "{\"detail\":\"Shared-projection tuning is managed internally by the Go projector pipeline.\",\"status\":\"not_applicable\"}\n"

// TestAdminTuningReportRouteFrozen pins the promoted surface: the
// single tuning-report GET the lane terminates. A route rename or
// rehome fails here until the ownership map, the spec fragment, and
// this mirror move together.
func TestAdminTuningReportRouteFrozen(t *testing.T) {
	t.Parallel()

	if adminTuningReportRoute != "GET /api/v0/admin/shared-projection/tuning-report" {
		t.Fatalf("adminTuningReportRoute = %q, want the frozen tuning-report GET", adminTuningReportRoute)
	}
}

// TestAdminTuningReportTerminatesLocally pins that the promoted route
// is an explicit registration ahead of the generic proxy: a shared-key
// call serves the pinned static body on the native lane with zero
// core contact — exact path and trailing-slash twin alike.
func TestAdminTuningReportTerminatesLocally(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey)
	counter := InstallCoreCallCounter(srv)
	srv.WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/admin/shared-projection/tuning-report",
		"/api/v0/admin/shared-projection/tuning-report/",
	} {
		req := authedRequest(http.MethodGet, target)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want the terminated 200", target, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("GET %s: Content-Type = %q, want the JSON document shape", target, got)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if string(body) != adminTuningPinnedBody {
			t.Errorf("GET %s: body = %q, want the pinned static body %q", target, body, adminTuningPinnedBody)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (shared-key terminates locally)", core.hits)
	}
	counter.AssertZero(t, "shared-key local termination")
	snap := tel.snapshot()
	if got := snap.authorizer["admin"][decisionAllow]; got != 2 {
		t.Errorf("authorizer[admin][allow] = %d, want 2 (exact path plus twin)", got)
	}
	lanes, ok := snap.dispatches["admin"]
	if !ok {
		t.Fatal("family admin has no dispatch rows, want the explicit promoted route ahead of relay")
	}
	if got := lanes[viaNative][outcomeLocalOK]; got != 2 {
		t.Errorf("family admin native local_ok = %d, want 2 (exact path plus twin)", got)
	}
	if got := snap.requests["admin"]["2xx"]; got != 2 {
		t.Errorf("requests[admin][2xx] = %d, want 2", got)
	}
}

// TestAdminTuningReportBodyByteIdentical pins the termination
// contract against the pinned core constants: the served bytes equal
// the apilib WriteTuningReport output byte for byte, the apilib
// constants equal the frozen literals, and the in-tree admission
// version still pins the same contract version.
func TestAdminTuningReportBodyByteIdentical(t *testing.T) {
	t.Parallel()

	if apilibadmin.TuningReportContractVersion != "v0" {
		t.Errorf("apilib contract version = %q, want v0", apilibadmin.TuningReportContractVersion)
	}
	if apilibadmin.TuningReportStatus != "not_applicable" {
		t.Errorf("apilib status = %q, want not_applicable", apilibadmin.TuningReportStatus)
	}
	if apilibadmin.TuningReportDetail != "Shared-projection tuning is managed internally by the Go projector pipeline." {
		t.Errorf("apilib detail drifted: %q", apilibadmin.TuningReportDetail)
	}
	if SharedProjectionTuningReportContractVersion != apilibadmin.TuningReportContractVersion {
		t.Errorf("in-tree contract version = %q, want the apilib %q", SharedProjectionTuningReportContractVersion, apilibadmin.TuningReportContractVersion)
	}

	direct := httptest.NewRecorder()
	apilibadmin.WriteTuningReport(direct, httptest.NewRequest(http.MethodGet, "/api/v0/admin/shared-projection/tuning-report", nil))
	if direct.Code != http.StatusOK {
		t.Fatalf("apilib direct status = %d, want 200", direct.Code)
	}
	if got := direct.Body.String(); got != adminTuningPinnedBody {
		t.Fatalf("apilib direct body = %q, want the pinned %q", got, adminTuningPinnedBody)
	}

	srv := NewServer("http://core.invalid", skeletonTestKey)
	rec := httptest.NewRecorder()
	req := authedRequest(http.MethodGet, "/api/v0/admin/shared-projection/tuning-report")
	srv.serveAdminTuningTerminated(rec, req)
	if got := rec.Body.String(); got != direct.Body.String() {
		t.Errorf("served body = %q, want the apilib bytes %q", got, direct.Body.String())
	}
}

// TestAdminTuningReportAuthorizerAllowFiled pins the family
// authorizer judgment on the promoted path: one shared-key call files
// one allow on the admin family, and the unreachable deny branch stays
// observable from dest signals.
func TestAdminTuningReportAuthorizerAllowFiled(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	tel := NewTelemetry(nil)
	srv.WithTelemetry(tel)

	rec := httptest.NewRecorder()
	req := authedRequest(http.MethodGet, "/api/v0/admin/shared-projection/tuning-report")
	srv.serveAdminTuningTerminated(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	snap := tel.snapshot()
	if got := snap.authorizer["admin"][decisionAllow]; got != 1 {
		t.Errorf("authorizer[admin][allow] = %d, want 1", got)
	}
	if got := snap.authorizer["admin"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[admin][deny] = %d, want 0 on the allow path", got)
	}
}

// TestAdminTuningReportDenyBranchShapePinned pins the deny shape for
// the day dest gains a scoped/session resolver: a non-AllScopes
// identity is denied with the documented family 403 in both negotiated
// shapes and the core is never contacted. The branch is unreachable
// on the live route today (the wave gate passes true); this pins the
// contract, not a live path.
func TestAdminTuningReportDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := authedRequest(http.MethodGet, "/api/v0/admin/shared-projection/tuning-report")
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		srv.serveAdminTuningTerminatedAs(rec, req, false)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Accept %q: status = %d, want 403", accept, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), d3FamilyDenyMessage) {
			t.Errorf("Accept %q: body = %q, want the family-403 message", accept, body)
		}
	}
}

// TestAdminTuningReportProofZeroCore is the zero-core proof for the
// promoted leaf: a P-FLOW case pairing the shared-key success with
// the credential-less denial through the full production chain with
// the counting transport. Every leg asserts zero core-origin calls
// and absolute store-span silence — the no-store class evidence.
func TestAdminTuningReportProofZeroCore(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	counter := InstallCoreCallCounter(srv)
	rec := &RecordingStoreSpans{}
	handler := withStoreSpanRecorder(rec, srv.Handler(DefaultV0Sunset))

	RunProof(t, handler, counter, rec, []ProofCase{
		{
			Name: "tuning local success then denial",
			Kind: ProofFlow,
			Requests: []ProofRequest{
				{Method: http.MethodGet, Path: "/api/v0/admin/shared-projection/tuning-report", Credential: "valid"},
				{Method: http.MethodGet, Path: "/api/v0/admin/shared-projection/tuning-report", Credential: "missing"},
			},
			WantStatuses: []int{http.StatusOK, http.StatusUnauthorized},
			WantSpans:    nil,
		},
	})
}

// TestAdminTuningReportDenial401ZeroCore pins that the promoted route
// keeps the dest 401 shape: credential-less callers get the envelope
// error or the plain error_code body with the bare Bearer challenge
// on both spellings — and the core is never contacted.
func TestAdminTuningReportDenial401ZeroCore(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/admin/shared-projection/tuning-report",
		"/api/v0/admin/shared-projection/tuning-report/",
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("GET %s Accept %q: status = %d, want 401", target, accept, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("GET %s Accept %q: WWW-Authenticate = %q, want bare Bearer", target, accept, got)
			}
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (gate denials never dispatch)", core.hits)
	}
	counter.AssertZero(t, "credential-less denial")
}

// TestAdminTuningReportForeignRelayExplicit pins the never-invent
// rule for the promoted route: a call carrying a credential class
// dest cannot resolve relays explicitly to the core on the relay
// lane — core judges, core bytes come back untouched — and never
// answers a dest-invented failure.
func TestAdminTuningReportForeignRelayExplicit(t *testing.T) {
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
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, "/api/v0/admin/shared-projection/tuning-report", nil)
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

// TestAdminTuningReportJunkCookieStays401 pins the D2 follow-up cookie
// lesson for the promoted route: only session-cookie-shaped
// credentials relay. Any other Cookie stays dest-401 with zero core
// contact, exactly like a credential-less caller, in both negotiated
// shapes.
func TestAdminTuningReportJunkCookieStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, cookie := range []string{
		"theme=dark",
		"unrelated=ignore; other=1",
		"eshu_session_stale=",
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodGet, "/api/v0/admin/shared-projection/tuning-report", nil)
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

// TestAdminTuningReportV1AliasServesLocal pins that the v1 alias
// rewrite lands the promoted route on the same local handler: the
// shared-key v1 call serves the pinned body with zero core contact.
func TestAdminTuningReportV1AliasServesLocal(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v1/admin/shared-projection/tuning-report")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != adminTuningPinnedBody {
		t.Errorf("body = %q, want the pinned static body", body)
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (v1 alias serves locally)", core.hits)
	}
	counter.AssertZero(t, "v1 alias local termination")
}

// TestAdminTuningReportCensus is the static-analysis half of the
// zero-core claim: the shared-key handler closure reaches no core
// sink. The foreign classes relay by design (pinned above), so the
// RouteClaims cover only the proven shared-key class (decision 4).
func TestAdminTuningReportCensus(t *testing.T) {
	t.Parallel()

	AssertNoCorePath(t, []RouteClaim{
		{Pattern: "GET /api/v0/admin/shared-projection/tuning-report", Handler: "Server.serveAdminTuningTerminated", Credential: "shared-key"},
	})
}

// TestAdminTuningReportSpecKeyServed pins that the promotion keeps the
// route's served-spec entry: keyed callers get the dest-assembled
// document with the tuning-report path key in both negotiated shapes,
// with or without a core origin configured.
func TestAdminTuningReportSpecKeyServed(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"http://core.invalid", ""} {
		srv := NewServer(origin, skeletonTestKey)
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
		item, ok := paths["/api/v0/admin/shared-projection/tuning-report"].(map[string]any)
		if !ok {
			t.Errorf("origin %q: served spec lacks the tuning-report key", origin)
			continue
		}
		if _, ok := item["get"].(map[string]any); !ok {
			t.Errorf("origin %q: served spec lacks get under the tuning-report key", origin)
		}
	}
}

// BenchmarkAdminTuningTerminated measures the promoted path for a
// shared-key identity GET through the full production chain. The stub
// core answers from memory but the lane never dials it — the numbers
// bound dest-side cost only (no pool checkout, no store, no core).
func BenchmarkAdminTuningTerminated(b *testing.B) {
	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	srv := NewServer(upstream.URL, skeletonTestKey)
	counter := InstallCoreCallCounter(srv)
	handler := srv.Handler(DefaultV0Sunset)

	warm := authedRequest(http.MethodGet, "/api/v0/admin/shared-projection/tuning-report")
	warmRec := httptest.NewRecorder()
	handler.ServeHTTP(warmRec, warm)
	if warmRec.Code != http.StatusOK {
		b.Fatalf("warmup status = %d, want 200", warmRec.Code)
	}
	if got := counter.Calls(); got != 0 {
		b.Fatalf("warmup core-origin calls = %d, want 0 (no pool, no core)", got)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, warm.Clone(warm.Context()))
	}
	if got := counter.Calls(); got != 0 {
		b.Fatalf("core-origin calls = %d, want 0 (no pool, no core)", got)
	}
}
