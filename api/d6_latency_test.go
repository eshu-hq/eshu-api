// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// D6 Status + SupplyChain latency evidence (Refs eshu-hq/eshu#9669,
// part of #6707). The terminated reader does the same single
// server-side fetch as the generic relay plus the wave gate and the
// family authorizer; these benchmarks bound that overhead per
// representative route so scripts/measure-d6-latency.sh can gate it in
// CI. The stub core answers from memory — the numbers isolate
// dest-side cost, not core or network time. Absolute deployment
// numbers ride the pinned-core-image leg at flip prep (D11).

// d6BenchServer builds a Handler over a stub core answering body
// for every route, so terminated and relay benchmarks share the same
// upstream shape.
func d6BenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkD6TerminatedRead measures the terminated path for a
// shared-key GET through the full production chain: one Status leg
// and one SupplyChain leg.
func BenchmarkD6TerminatedRead(b *testing.B) {
	_, srv := d6BenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	statusGet := authedRequest(http.MethodGet, "/api/v0/status/pipeline")
	supplyGet := authedRequest(http.MethodGet, "/api/v0/supply-chain/impact/findings")

	b.Run("status", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, statusGet.Clone(statusGet.Context()))
		}
	})
	b.Run("supply", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, supplyGet.Clone(supplyGet.Context()))
		}
	})
}

// BenchmarkD6RelayRead measures the generic relay path for the
// same caller and stub shape: the before leg the lane must not
// regress. The legs compare different routes by necessity (the D6
// paths now terminate, and the D2-D4 relay legs moved to the same
// still-relaying routes); both do exactly one server-side fetch, so
// the 1.50 bound stays a real pathology check.
func BenchmarkD6RelayRead(b *testing.B) {
	_, srv := d6BenchServer(b, `{"data":[],"error":null,"truth":null}`)
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
