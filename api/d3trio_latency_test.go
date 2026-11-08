// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// D3 Gap 02 trio latency evidence (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#70). The terminated reader does the same
// single-server-side fetch as the generic relay plus the wave gate
// and the family authorizer; these benchmarks bound that overhead per
// trio route so the lane can report per-route numbers. The stub core
// answers from memory — the numbers isolate dest-side cost, not core
// or network time. Absolute deployment numbers ride the
// pinned-core-image leg at flip prep (D11).

// BenchmarkD3TrioTerminatedRead measures the terminated path per trio
// route for a shared-key identity GET through the full production
// chain.
func BenchmarkD3TrioTerminatedRead(b *testing.B) {
	core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range []struct{ name, target string }{
		{"delta-union-completeness", "/api/v0/admin/delta-union/completeness?scope=s1"},
		{"reindex-progress", "/api/v0/admin/reindex/progress?scope_id=s1"},
		{"tuning-report", "/api/v0/admin/shared-projection/tuning-report"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			req := authedRequest(http.MethodGet, tc.target)
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req.Clone(req.Context()))
			}
		})
	}
}
