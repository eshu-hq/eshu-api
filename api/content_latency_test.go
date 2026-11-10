// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Wave-2 Content latency evidence (Refs eshu-hq/eshu#9669, D13
// tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#72). The
// terminated reader does the same single server-side fetch as the
// generic relay plus the wave gate and the family authorizer; these
// benchmarks bound that overhead per representative route so
// scripts/measure-content-latency.sh can gate it in CI. The stub core
// answers from memory — the numbers isolate dest-side cost, not core
// or network time. Absolute deployment numbers ride the pinned-core
// image leg at flip prep (D11).

// contentBenchServer builds a Handler over a stub core answering body
// for every route, so terminated and relay benchmarks share the same
// upstream shape.
func contentBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkContentTerminatedRead measures the terminated path for a
// shared-key POST through the full production chain: one leg per wave
// route through the same wave gate and family authorizer.
func BenchmarkContentTerminatedRead(b *testing.B) {
	_, srv := contentBenchServer(b, `{"repo_id":"repo-1"}`)
	handler := srv.Handler(DefaultV0Sunset)

	legs := map[string]struct {
		path string
		body string
	}{
		"files-read":    {"/api/v0/content/files/read", `{"repo_id":"repo-1","relative_path":"src/app.go"}`},
		"files-lines":   {"/api/v0/content/files/lines", `{"repo_id":"repo-1","relative_path":"src/app.go","start_line":2,"end_line":4}`},
		"entities-read": {"/api/v0/content/entities/read", `{"entity_id":"entity-1"}`},
	}
	for name, leg := range legs {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, contentAuthedPost(leg.path, leg.body, "application/json"))
			}
		})
	}
}

// BenchmarkContentRelayRead measures the generic relay path for the
// same caller and stub shape: the before leg the lane must not
// regress. The legs compare different routes by necessity (the wave
// paths now terminate; the out-of-pin file search POST still relays);
// both do exactly one server-side fetch, so the 1.50 bound stays a
// real pathology check.
func BenchmarkContentRelayRead(b *testing.B) {
	_, srv := contentBenchServer(b, `{"results":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, contentAuthedPost("/api/v0/content/files/search", `{"query":"render"}`, "application/json"))
		}
	})
}
