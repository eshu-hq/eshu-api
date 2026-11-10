// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// W7-read admin sign-in-policy GET conformance (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#72). TDD record: this suite failed RED before the
// family lane landed (the family wave table and the terminated
// handler did not exist, so the frozen and fail-closed assertions
// failed at compile; the shared-key legs rode the generic LeafB
// gate, so the family-authorizer assertions failed) and is green
// with it. The want table mirrors the CUTOVER_OWNERSHIP
// SignInPolicyReads row behind SignInPolicyReadAdmission v0, and the
// behavior tests require the route to terminate natively for
// shared-key callers on the family gate while unresolvable identity
// classes relay explicitly. Serving semantics match the LeafB gate
// the route rides today bit for bit (same credential matrix,
// same forward, same bytes) — the lane changes ownership and
// proof, not behavior.

// TestAdminSignInPolicyReadWaveFrozen pins the exact W7-read surface:
// the single admin GET the family lane terminates. A wave change
// (route added, removed, renamed, or rehomed) fails here until the
// ownership map, the spec fragment, and this mirror move together.
func TestAdminSignInPolicyReadWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/auth/admin/sign-in-policy sign_in_policy wave-family paths/auth/sign_in_policy.go",
	}
	var got []string
	for _, entry := range adminSignInPolicyReadWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("adminSignInPolicyReadWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("adminSignInPolicyReadWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAdminSignInPolicyReadTerminatesAheadOfRelay pins that the admin
// GET is an explicit family registration ahead of the generic proxy:
// a shared-key call must file its dispatch on the native lane
// (viaNative) with one family-authorizer allow judgment. This test
// is the swap-mutant detector for dispatch: if the family
// registration is swapped back to the generic dispatch, the leg
// files relay and this test goes RED.
func TestAdminSignInPolicyReadTerminatesAheadOfRelay(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"tenant_id":"tenant_a","require_sso":false}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/auth/admin/sign-in-policy/",
	} {
		req := authedRequest(http.MethodGet, target)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200 from the stub core", target, rec.Code)
		}
	}
	snap := tel.snapshot()
	if got := snap.authorizer["sign_in_policy"][decisionAllow]; got != 2 {
		t.Errorf("authorizer[sign_in_policy][allow] = %d, want 2 (exact path plus twin)", got)
	}
	lanes, ok := snap.dispatches["sign_in_policy"]
	if !ok {
		t.Fatal("family sign_in_policy has no dispatch rows, want the explicit family route ahead of relay")
	}
	if lanes[viaNative] == nil {
		t.Errorf("family sign_in_policy dispatches = %v, want a native lane row for the shared-key family call", lanes)
	}
	if got := lanes[viaNative][outcomeCoreOK]; got != 2 {
		t.Errorf("family sign_in_policy native core_ok = %d, want 2", got)
	}
}

// TestAdminSignInPolicyUnknownRouteFailClosed pins the terminated
// handler switch default: any route the family table does not know
// answers 404 and never rides the leg. Routes() only passes family
// wave entries, so this guards the future table-add-without-a-handler
// mistake.
func TestAdminSignInPolicyUnknownRouteFailClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, route := range []string{
		"GET /api/v0/auth/admin/sign-in-policy/bogus",
		"PATCH /api/v0/auth/admin/sign-in-policy",
		"GET /api/v0/auth/sign-in-policy",
		"",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/admin/sign-in-policy", nil)
		srv.adminSignInPolicyTerminatedHandler(route)(rec, req)

		res := rec.Result()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("route %q: status = %d, want the fail-closed 404", route, res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if len(body) == 0 {
			t.Errorf("route %q: empty body, want the documented unknown-route error", route)
		}
	}
}

// TestAdminSignInPolicyAuthorizerAllowFiled pins the family
// authorizer judgment on the terminated path: one shared-key call
// files one allow on the sign_in_policy family, and the unreachable
// deny branch stays observable.
func TestAdminSignInPolicyAuthorizerAllowFiled(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"tenant_id":"tenant_a","require_sso":false}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v0/auth/admin/sign-in-policy"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	snap := tel.snapshot()
	if got := snap.authorizer["sign_in_policy"][decisionAllow]; got != 1 {
		t.Errorf("authorizer[sign_in_policy][allow] = %d, want 1", got)
	}
	if got := snap.authorizer["sign_in_policy"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[sign_in_policy][deny] = %d, want 0 on the allow path", got)
	}
}

// TestAdminSignInPolicyTerminatedAs_NilCore503 pins the fail-closed
// origin: with no core origin configured the family reader answers
// the permanent 503 backend_unavailable instead of panicking, in
// both negotiated shapes and for both spellings.
func TestAdminSignInPolicyTerminatedAs_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatal("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		for _, path := range []string{
			"/api/v0/auth/admin/sign-in-policy",
			"/api/v0/auth/admin/sign-in-policy/",
		} {
			req := authedRequest(http.MethodGet, path)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("Accept %q path %q: panicked with %v, want 503", accept, path, recovered)
					}
				}()
				srv.serveAdminSignInPolicyTerminatedAs(rec, req, true)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("Accept %q path %q: status = %d, want 503", accept, path, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Accept %q path %q: Retry-After = %q, want absent (permanent 503)", accept, path, got)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), backendUnavailableCode) {
				t.Errorf("Accept %q path %q: body = %q, want backend_unavailable", accept, path, body)
			}
		}
	}
}

// TestAdminSignInPolicyTerminatedAs_Deny403 pins the deny shape for
// the day dest gains a scoped/session resolver: a non-AllScopes
// identity is denied with the documented family 403 in both
// negotiated shapes and the core is never reached. The branch is
// unreachable on the live routes today (the family gate passes true;
// the live 403s are core's), so this pins the contract, not a live
// path.
func TestAdminSignInPolicyTerminatedAs_Deny403(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"tenant_id":"tenant_a"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := authedRequest(http.MethodGet, "/api/v0/auth/admin/sign-in-policy")
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		srv.serveAdminSignInPolicyTerminatedAs(rec, req, false)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Accept %q: status = %d, want 403", accept, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "all-scopes operator authorization is required") {
			t.Errorf("Accept %q: body = %q, want the family-403 message", accept, body)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (a deny never dispatches)", core.hits)
	}
}

// TestAdminSignInPolicyReadGate_EmptyKey401 pins that an empty
// configured key fails closed like RequireAuth: every credential
// shape keeps the standing 401 with zero core contact.
func TestAdminSignInPolicyReadGate_EmptyKey401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"tenant_id":"tenant_a"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, "")
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/auth/admin/sign-in-policy/",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest(http.MethodGet, target))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s: status = %d, want 401 (empty key fails closed)", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0", core.hits)
	}
}

// TestAdminSignInPolicyRelayExplicit_NilCore503 pins that the
// explicit-relay leg fails closed the same way when no core origin
// is configured: the permanent 503, never a panic.
func TestAdminSignInPolicyRelayExplicit_NilCore503(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/admin/sign-in-policy", nil)
	req.Header.Set("Authorization", "Bearer scoped-token")
	rec := httptest.NewRecorder()

	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("panicked with %v, want 503", recovered)
			}
		}()
		srv.adminSignInPolicyRelayExplicit(rec, req)
	}()

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}
