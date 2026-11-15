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

// Impact latency evidence (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#97). The terminated reader does the same single
// server-side fetch as the generic relay plus the wave gate and the
// family authorizer; these benchmarks bound that overhead for the 11
// proceed legs plus the two adapter holds. The stub core answers from
// memory — the numbers isolate dest-side cost, not core or network
// time. Absolute deployment numbers ride the pinned-core-image leg at
// flip prep (D11).

// impactBenchServer builds a Handler over a stub core answering body
// for every route, so terminated and relay benchmarks share the same
// upstream shape.
func impactBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkImpactTerminatedRead measures the terminated path for
// shared-key impact calls through the full production chain: one leg
// per gated wave row through the same wave gate and family authorizer.
func BenchmarkImpactTerminatedRead(b *testing.B) {
	_, srv := impactBenchServer(b, `{"data":[],"error":null,"truth":null}`)
	handler := srv.Handler(DefaultV0Sunset)

	seen := map[string]bool{}
	for _, leg := range impactParityLegs {
		if impactWaveEntryForBench(leg.key).Serve != impactServeGated {
			continue
		}
		if seen[leg.target] {
			continue
		}
		seen[leg.target] = true
		body := leg.body
		target := leg.target
		b.Run(strings.ReplaceAll(strings.TrimPrefix(target, "/api/v0/impact/"), "/", "-"), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				post := authedRequest(http.MethodPost, target)
				post.Body = io.NopCloser(strings.NewReader(body))
				post.ContentLength = int64(len(body))
				post.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, post)
			}
		})
	}
}

// impactWaveEntryForBench resolves a wave key without *testing.T (the
// benchmark path never misses: the keys come from the wave legs).
func impactWaveEntryForBench(key string) impactRoute {
	for _, entry := range impactWave {
		if entry.Method+" "+entry.Path == key {
			return entry
		}
	}
	return impactRoute{}
}

// BenchmarkImpactRelayRead measures the relay path for the same caller
// shape: the two adapter holds (shared-key relay on the explicit relay
// stack) plus still-relayed routes no wave table covers, which cross
// the generic proxy. Each does exactly one server-side fetch, so the
// 1.50 bound stays a real pathology check.
func BenchmarkImpactRelayRead(b *testing.B) {
	_, srv := impactBenchServer(b, `{"data":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	relayPosts := []struct {
		name   string
		target string
		body   string
	}{
		{"hold-change-surface", "/api/v0/impact/change-surface", `{"repo_id":"repo_1"}`},
		{"hold-change-surface-investigate", "/api/v0/impact/change-surface/investigate", `{"repo_id":"repo_1"}`},
		{"post-relay", "/api/v0/code/bundles", `{"scope_id":"scope_1"}`},
	}
	for _, leg := range relayPosts {
		b.Run(leg.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				post := authedRequest(http.MethodPost, leg.target)
				post.Body = io.NopCloser(strings.NewReader(leg.body))
				post.ContentLength = int64(len(leg.body))
				post.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, post)
			}
		})
	}
}
