// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// W7-read admin sign-in-policy GET latency evidence (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#72). The terminated reader does the same single
// server-side fetch as the generic relay plus the family gate and
// the family authorizer; these benchmarks bound that overhead for
// both spellings so
// scripts/measure-signinpolicy-admin-read-latency.sh can gate it in
// CI. The stub core answers from memory — the numbers isolate
// dest-side cost, not core or network time. Absolute deployment
// numbers ride the pinned-core-image leg at flip prep (D11).

// adminSignInPolicyBenchServer builds a Handler over a stub core
// answering body for every route, so terminated and relay benchmarks
// share the same upstream shape.
func adminSignInPolicyBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkAdminSignInPolicyTerminatedRead measures the terminated
// path for a shared-key GET through the full production chain: one
// leg per spelling through the same family gate and family
// authorizer.
func BenchmarkAdminSignInPolicyTerminatedRead(b *testing.B) {
	_, srv := adminSignInPolicyBenchServer(b, `{"tenant_id":"tenant_a","require_sso":false}`)
	handler := srv.Handler(DefaultV0Sunset)

	gets := map[string]*http.Request{
		"exact": authedRequest(http.MethodGet, "/api/v0/auth/admin/sign-in-policy"),
		"twin":  authedRequest(http.MethodGet, "/api/v0/auth/admin/sign-in-policy/"),
	}
	for name, get := range gets {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, get.Clone(get.Context()))
			}
		})
	}
}

// BenchmarkAdminSignInPolicyRelayRead measures the generic relay
// path for the same caller and stub shape: the before leg the lane
// must not regress. The legs compare different routes by necessity
// (the admin path now terminates); both do exactly one server-side
// fetch, so the 1.50 bound stays a real pathology check.
func BenchmarkAdminSignInPolicyRelayRead(b *testing.B) {
	_, srv := adminSignInPolicyBenchServer(b, `{"tenant_id":"tenant_a","require_sso":false}`)
	handler := srv.Handler(DefaultV0Sunset)

	get := authedRequest(http.MethodGet, "/api/v0/dependencies")

	b.Run("get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, get.Clone(get.Context()))
		}
	})
}
