// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// G2 latency evidence (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101 RE:G2V2). The terminated reader does the same
// single server-side fetch as the generic relay plus the wave gate
// and the family authorizer; these benchmarks bound that overhead
// per representative route so scripts/measure-g2-latency.sh
// gates it in CI. The stub core answers from memory — the numbers
// isolate dest-side cost, not core or network time. Absolute
// deployment numbers ride the pinned-core image leg at flip prep
// (D11).

// g2BenchServer builds a Handler over a stub core answering body for
// every route, so terminated and relay benchmarks share the same
// upstream shape.
func g2BenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkG2TerminatedRead measures the terminated path for a
// shared-key call through the full production chain: one leg per
// wave route through the same wave gate and family authorizer.
func BenchmarkG2TerminatedRead(b *testing.B) {
	_, srv := g2BenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	gets := map[string]*http.Request{
		"readiness":             g2AuthedReq(http.MethodGet, "/api/v0/collector-extraction-readiness", ""),
		"readiness-family":      g2AuthedReq(http.MethodGet, "/api/v0/collector-extraction-readiness/repo", ""),
		"fact-schema":           g2AuthedReq(http.MethodGet, "/api/v0/fact-schema-versions", ""),
		"fact-schema-kind":      g2AuthedReq(http.MethodGet, "/api/v0/fact-schema-versions/file", ""),
		"surface-inventory":     g2AuthedReq(http.MethodGet, "/api/v0/surface-inventory", ""),
		"visualizations-derive": g2AuthedReq(http.MethodPost, "/api/v0/visualizations/derive", `{"packet":"derive-probe"}`),
		"manifest-dependencies": g2AuthedReq(http.MethodGet, "/api/v0/repository/manifest-dependencies", ""),
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

// BenchmarkG2RelayRead measures the generic relay path for the same
// caller and stub shape: the before leg the lane must not regress.
// The legs compare different routes by necessity (the G2 paths now
// terminate); both do exactly one server-side fetch, so the 1.50
// bound stays a real pathology check.
func BenchmarkG2RelayRead(b *testing.B) {
	_, srv := g2BenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	get := g2AuthedReq(http.MethodGet, "/api/v0/dependencies", "")

	b.Run("get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, get.Clone(get.Context()))
		}
	})
}
