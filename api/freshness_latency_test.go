// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Wave-1 Freshness latency evidence (Refs eshu-hq/eshu#9669, D13
// tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#72). The
// terminated reader does the same single server-side fetch as the
// generic relay plus the wave gate and the family authorizer; these
// benchmarks bound that overhead per representative route so
// scripts/measure-freshness-latency.sh can gate it in CI. The stub core
// answers from memory — the numbers isolate dest-side cost, not core
// or network time. Absolute deployment numbers ride the pinned-core
// image leg at flip prep (D11).

// freshnessBenchServer builds a Handler over a stub core answering
// body for every route, so terminated and relay benchmarks share the
// same upstream shape.
func freshnessBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkFreshnessTerminatedRead measures the terminated path for a
// shared-key GET through the full production chain: one leg per wave
// route through the same wave gate and family authorizer.
func BenchmarkFreshnessTerminatedRead(b *testing.B) {
	_, srv := freshnessBenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	gets := map[string]*http.Request{
		"changed-since":         authedRequest(http.MethodGet, "/api/v0/freshness/changed-since?scope_id=scope_1&since_generation_id=gen_9"),
		"generations":           authedRequest(http.MethodGet, "/api/v0/freshness/generations?scope_id=scope_1&limit=10"),
		"service-changed-since": authedRequest(http.MethodGet, "/api/v0/freshness/services/changed-since?service_id=svc_1&since_generation_id=gen_9"),
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

// BenchmarkFreshnessRelayRead measures the generic relay path for the
// same caller and stub shape: the before leg the lane must not
// regress. The legs compare different routes by necessity (the
// Wave-1 paths now terminate); both do exactly one server-side fetch,
// so the 1.50 bound stays a real pathology check.
func BenchmarkFreshnessRelayRead(b *testing.B) {
	_, srv := freshnessBenchServer(b, `{"data":[],"error":null,"truth":null}`)
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
