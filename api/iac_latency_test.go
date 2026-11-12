// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// W3 IaC-6 latency evidence (Refs eshu-hq/eshu-api#102, part of
// eshu-hq/eshu#9669). The native legs do the same single server-side
// fetch as the generic relay plus the explicit route match and
// per-family telemetry; these benchmarks bound that overhead per leg
// so scripts/measure-iac-latency.sh can gate it in CI. The stub core
// answers from memory — the numbers isolate dest-side cost, not core
// or network time. Absolute deployment numbers ride the
// pinned-core-image leg at flip prep (D11).

// iacBenchServer builds a Handler over a stub core answering body for
// every route, so native and relay benchmarks share the same upstream
// shape.
func iacBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkIaCNativeRead measures the four cleared legs through the
// full production chain: one shared-key POST per leg.
func BenchmarkIaCNativeRead(b *testing.B) {
	_, srv := iacBenchServer(b, `{"data":{"total":0},"error":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	bench := func(name, path string) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			req := authedRequest(http.MethodPost, path)
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req.Clone(req.Context()))
			}
		})
	}
	bench("unmanaged-resources", iacRoutePath(iacUnmanagedResourcesRoute))
	bench("management-status", iacRoutePath(iacManagementStatusRoute))
	bench("management-status-explain", iacRoutePath(iacManagementStatusExplainRoute))
	bench("terraform-import-plan-candidates", iacRoutePath(iacTerraformImportPlanCandidatesRoute))
}

// BenchmarkIaCRelayRead measures the relay leg through the full
// production chain: POST /api/v0/iac/dead stays on the generic relay,
// so it is the before-shape for every cleared leg above. Both legs do
// exactly one server-side fetch, so the 1.50 bound stays a real
// pathology check either way.
func BenchmarkIaCRelayRead(b *testing.B) {
	_, srv := iacBenchServer(b, `{"data":{"total":0},"error":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("dead-relay", func(b *testing.B) {
		b.ReportAllocs()
		req := authedRequest(http.MethodPost, "/api/v0/iac/dead")
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req.Clone(req.Context()))
		}
	})
}
