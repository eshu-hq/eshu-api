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

// D6 Status + SupplyChain terminated-vs-relay byte parity (Refs
// eshu-hq/eshu#9669, part of #6707). The terminated reader must not
// distort the core's answer: for every stub-core outcome its
// client-visible bytes must equal the generic byte-relay's. This
// guards the dest-owned response mapping (forwardToCore on the native
// lane) against future divergence from the relay contract it mirrors.

// d6ParityRoutes covers one concrete path per gated wave family for
// the parity matrix. The SupplyChain list entries carry the Decision
// 2 suppression-state filters (suppression_state,
// include_suppressed, provider_state, reconciliation_status): parity
// must exercise suppression-state filtering or the split is unproven,
// so the matrix drives those queries through the native lane and
// TestD6SuppressionFiltersForwarded pins they arrive intact.
var d6ParityRoutes = []string{
	"/api/v0/collector-readiness",
	"/api/v0/collectors",
	"/api/v0/ingesters",
	"/api/v0/ingesters/repository",
	"/api/v0/status/collector-readiness",
	"/api/v0/status/collectors",
	"/api/v0/status/freshness-causality",
	"/api/v0/status/ingesters/repository",
	"/api/v0/status/operator-control-plane",
	"/api/v0/status/pipeline",
	"/api/v0/status/semantic-extraction",
	"/api/v0/supply-chain/advisories",
	"/api/v0/supply-chain/advisories/evidence",
	"/api/v0/supply-chain/container-images/identities",
	"/api/v0/supply-chain/container-images/identities/count",
	"/api/v0/supply-chain/container-images/identities/inventory",
	"/api/v0/supply-chain/impact/explain?subject_digest=sha256:abc",
	"/api/v0/supply-chain/impact/findings?repository_id=r1&suppression_state=active",
	"/api/v0/supply-chain/impact/findings?repository_id=r1&suppression_state=not_affected&include_suppressed=true",
	"/api/v0/supply-chain/impact/findings/count?repository_id=r1&suppression_state=ignored",
	"/api/v0/supply-chain/impact/inventory?repository_id=r1&include_suppressed=true",
	"/api/v0/supply-chain/sbom-attestations/attachments",
	"/api/v0/supply-chain/sbom-attestations/attachments/count",
	"/api/v0/supply-chain/sbom-attestations/attachments/inventory",
	"/api/v0/supply-chain/security-alerts/reconciliations?repository_id=r1&provider_state=open&reconciliation_status=pending",
	"/api/v0/supply-chain/security-alerts/reconciliations/count?repository_id=r1&provider_state=fixed",
	"/api/v0/supply-chain/security-alerts/reconciliations/inventory?repository_id=r1&reconciliation_status=acknowledged",
	"/api/v0/supply-chain/vulnerabilities/GHSA-1234-5678-abcd",
	"/api/v0/supply-chain/vulnerability-scanner/contract",
}

// TestD6TerminatedMatchesRelayByteForByte pins that the terminated
// reader relays core bytes exactly like the generic proxy: same
// status, same headers (modulo per-hop framing), same body — across
// the pinned outcome matrix in both negotiated shapes. The matrix runs
// every gated wave family; the stub answers the pinned wire strings
// where core owns the text, and opaque bytes where the relay (not the
// text) is what this test pins.
func TestD6TerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"data":[],"error":null,"truth":null}`},
		{"markdown 200", http.StatusOK, http.Header{"Content-Type": []string{"text/markdown; charset=utf-8"}}, "# packet\n"},
		{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"invalid search request"}`},
		{"core all-scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`},
		{"tenant scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"admin tenant scope is required"}`},
		{"missing 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"repository not found"}`},
		{"ambiguous 409", http.StatusConflict, nil, `{"error":"ambiguous","detail":"two services match"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"reader is not configured"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"reader fenced"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/api/v0/auth/oidc/login"}}, ""},
	}
	for _, route := range d6ParityRoutes {
		for _, outcome := range outcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				name := "GET " + route + " " + outcome.name + " " + accept
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					nativeCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
					nativeMux := http.NewServeMux()
					nativeMux.Handle("/", nativeCore.handler())
					nativeUpstream := httptest.NewServer(nativeMux)
					defer nativeUpstream.Close()

					relayCore := &stubCore{status: outcome.status, header: relayHeaders(outcome.header), body: outcome.body}
					relayMux := http.NewServeMux()
					relayMux.Handle("/", relayCore.handler())
					relayUpstream := httptest.NewServer(relayMux)
					defer relayUpstream.Close()

					build := func(target string) *http.Request {
						req := authedRequest(http.MethodGet, target)
						req.Header.Set("Accept", accept)
						req.Header.Set("X-Correlation-ID", "d6-parity-1")
						return req
					}

					nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
					nativeRec := httptest.NewRecorder()
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, build(route))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					// The relay side crosses the same middleware chain
					// the router builds for the generic proxy
					// (instrumented + key gate), so correlation echo,
					// deprecation headers, and lane filing match the
					// production relay path exactly.
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, build(route))

					nativeRes, relayRes := nativeRec.Result(), relayRec.Result()
					if nativeRes.StatusCode != relayRes.StatusCode {
						t.Fatalf("native status = %d, relay status = %d, want identical", nativeRes.StatusCode, relayRes.StatusCode)
					}
					nativeBody, _ := io.ReadAll(nativeRes.Body)
					relayBody, _ := io.ReadAll(relayRes.Body)
					if string(nativeBody) != string(relayBody) {
						t.Errorf("native body = %q, relay body = %q, want identical", nativeBody, relayBody)
					}
					for _, header := range []string{"Content-Type", "Retry-After", "Location", "Cache-Control", "X-Correlation-ID"} {
						if nativeRes.Header.Get(header) != relayRes.Header.Get(header) {
							t.Errorf("header %s: native = %q, relay = %q, want identical", header, nativeRes.Header.Get(header), relayRes.Header.Get(header))
						}
					}
					if nativeCore.lastQuery != relayCore.lastQuery {
						t.Errorf("upstream query: native = %q, relay = %q, want identical", nativeCore.lastQuery, relayCore.lastQuery)
					}
				})
			}
		}
	}
}

// TestD6SuppressionFiltersForwarded is the Decision 2 proof
// obligation: the finding reads consult suppression state only inside
// reader-store SQL — FindingFilter SuppressionState/IncludeSuppressed
// and the reconciliation provider-state/reconciliation-status filters
// — never via the mutation port. The native lane must forward those
// filter queries to the core untouched, so the split (reads native,
// suppressions row relayed via SuppressionAdmission) is proven and not
// merely asserted.
func TestD6SuppressionFiltersForwarded(t *testing.T) {
	t.Parallel()

	filters := []string{
		"/api/v0/supply-chain/impact/findings?repository_id=r1&suppression_state=active",
		"/api/v0/supply-chain/impact/findings?repository_id=r1&suppression_state=not_affected&include_suppressed=true",
		"/api/v0/supply-chain/impact/findings?repository_id=r1&suppression_state=expired&include_suppressed=false",
		"/api/v0/supply-chain/impact/findings/count?repository_id=r1&suppression_state=provider_dismissed",
		"/api/v0/supply-chain/impact/inventory?repository_id=r1&suppression_state=scope_mismatch&include_suppressed=true",
		"/api/v0/supply-chain/security-alerts/reconciliations?repository_id=r1&provider_state=open&reconciliation_status=pending",
		"/api/v0/supply-chain/security-alerts/reconciliations/count?repository_id=r1&provider_state=fixed",
		"/api/v0/supply-chain/security-alerts/reconciliations/inventory?repository_id=r1&reconciliation_status=acknowledged",
	}
	for _, target := range filters {
		t.Run("GET "+target, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			rec := httptest.NewRecorder()
			srv.Handler(DefaultV0Sunset).ServeHTTP(rec, authedRequest(http.MethodGet, target))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			want := strings.SplitN(target, "?", 2)[1]
			if core.lastQuery != want {
				t.Errorf("upstream query = %q, want the filter query %q forwarded intact", core.lastQuery, want)
			}
		})
	}
}
