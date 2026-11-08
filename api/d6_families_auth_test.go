// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// D6 Status + SupplyChain per-family authorizer proof (Refs
// eshu-hq/eshu#9669, part of #6707). Scope-honesty label: the dest
// authorizer covers the shared-key (AllScopes) identity only — the
// single class dest can construct today. Scoped tokens, OIDC bearers,
// and browser sessions relay explicitly (core judges;
// TestD6WaveRelayExplicit... pins that), and the deny branch below is
// unreachable on live routes until the D8 session resolver lands. Core
// re-judges every forwarded call as backstop (tenant scope, permission
// catalog, governance mode), which is where the live 403s come from.

// TestD6AuthorizerSharedKeyAllowDeny pins the family gate: the
// shared-key identity allows, anything else denies. One shared
// authorizer covers all 32 D6 wave routes until D8 resolves per-class
// handling.
func TestD6AuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeD6SharedKey(true) {
		t.Errorf("authorizeD6SharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeD6SharedKey(false) {
		t.Errorf("authorizeD6SharedKey(false) = allow, want deny (family 403)")
	}
}

// TestD6AuthorizerCountersPinned pins the authorizer telemetry for
// the wave: a terminated shared-key call files one allow judgment on
// its family, and the unreachable deny branch stays observable.
func TestD6AuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{"/api/v0/status/pipeline", "/api/v0/supply-chain/impact/findings"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest(http.MethodGet, target))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200", target, rec.Code)
		}
	}
	snap := tel.snapshot()
	if got := snap.authorizer["status"][decisionAllow]; got != 1 {
		t.Errorf("authorizer[status][allow] = %d, want 1", got)
	}
	if got := snap.authorizer["status"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[status][deny] = %d, want 0 on the allow path", got)
	}
	if got := snap.authorizer["supply_chain"][decisionAllow]; got != 1 {
		t.Errorf("authorizer[supply_chain][allow] = %d, want 1", got)
	}
	if got := snap.authorizer["supply_chain"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[supply_chain][deny] = %d, want 0 on the allow path", got)
	}
}

// TestD6DenyBranchShapePinned pins the deny shape for the day dest
// gains a scoped/session resolver: a non-AllScopes identity is denied
// with the documented family 403 in both negotiated shapes and the
// core is never reached. The branch is unreachable on the live routes
// today (the wave gate passes true; the live 403s are core's), so this
// pins the contract, not a live path.
func TestD6DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v0/collector-readiness"},
		{http.MethodGet, "/api/v0/collectors"},
		{http.MethodGet, "/api/v0/ingesters"},
		{http.MethodGet, "/api/v0/ingesters/repo_1"},
		{http.MethodGet, "/api/v0/status/collector-readiness"},
		{http.MethodGet, "/api/v0/status/collectors"},
		{http.MethodGet, "/api/v0/index-status"},
		{http.MethodGet, "/api/v0/status/freshness-causality"},
		{http.MethodGet, "/api/v0/status/index"},
		{http.MethodGet, "/api/v0/status/ingesters"},
		{http.MethodGet, "/api/v0/status/ingesters/repo_1"},
		{http.MethodGet, "/api/v0/status/operations"},
		{http.MethodGet, "/api/v0/status/operator-control-plane"},
		{http.MethodGet, "/api/v0/status/pipeline"},
		{http.MethodGet, "/api/v0/status/semantic-extraction"},
		{http.MethodGet, "/api/v0/supply-chain/advisories"},
		{http.MethodGet, "/api/v0/supply-chain/advisories/evidence"},
		{http.MethodGet, "/api/v0/supply-chain/container-images/identities"},
		{http.MethodGet, "/api/v0/supply-chain/container-images/identities/count"},
		{http.MethodGet, "/api/v0/supply-chain/container-images/identities/inventory"},
		{http.MethodGet, "/api/v0/supply-chain/impact/explain"},
		{http.MethodGet, "/api/v0/supply-chain/impact/findings"},
		{http.MethodGet, "/api/v0/supply-chain/impact/findings/count"},
		{http.MethodGet, "/api/v0/supply-chain/impact/inventory"},
		{http.MethodGet, "/api/v0/supply-chain/sbom-attestations/attachments"},
		{http.MethodGet, "/api/v0/supply-chain/sbom-attestations/attachments/count"},
		{http.MethodGet, "/api/v0/supply-chain/sbom-attestations/attachments/inventory"},
		{http.MethodGet, "/api/v0/supply-chain/security-alerts/reconciliations"},
		{http.MethodGet, "/api/v0/supply-chain/security-alerts/reconciliations/count"},
		{http.MethodGet, "/api/v0/supply-chain/security-alerts/reconciliations/inventory"},
		{http.MethodGet, "/api/v0/supply-chain/vulnerabilities/GHSA-1234-5678-abcd"},
		{http.MethodGet, "/api/v0/supply-chain/vulnerability-scanner/contract"},
	}
	for _, tc := range cases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := tc.method + " " + tc.path + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := authedRequest(tc.method, tc.path)
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				srv.serveD6TerminatedAs(rec, req, false)

				res := rec.Result()
				if res.StatusCode != http.StatusForbidden {
					t.Fatalf("status = %d, want the documented 403", res.StatusCode)
				}
				body, _ := io.ReadAll(res.Body)
				if !strings.Contains(string(body), d6FamilyDenyMessage) {
					t.Errorf("body = %q, want it to carry the documented deny text", body)
				}
			})
		}
	}
}
