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

// Workflow-retire-unclaimed clone-latency evidence (Refs
// eshu-hq/eshu#9669, part of #6707). The tier-C clone does the same
// single server-side fetch as the generic relay plus the explicit
// route match and the family authorizer; this benchmark bounds that
// overhead so scripts/measure-retireunclaimed-latency.sh can gate it
// in CI. The stub core answers from memory — the numbers isolate
// dest-side cost, not core or network time. Absolute deployment numbers
// ride the pinned-core-image leg at flip prep (D11).

// retireUnclaimedBenchServer builds a Handler over a stub core
// answering body for every route, so clone and relay benchmarks share
// the same upstream shape.
func retireUnclaimedBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// retireUnclaimedBenchPost builds one shared-key retire-unclaimed POST
// with the representative caller body, rebuilt per iteration since the
// server consumes it.
func retireUnclaimedBenchPost(body string) *http.Request {
	post := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed")
	post.Body = io.NopCloser(strings.NewReader(body))
	post.ContentLength = int64(len(body))
	return post
}

// BenchmarkRetireUnclaimedNativeWrite measures the clone path for a
// shared-key retire-unclaimed POST through the full production chain.
func BenchmarkRetireUnclaimedNativeWrite(b *testing.B) {
	_, srv := retireUnclaimedBenchServer(b, `{"count":0,"items":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("retire", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, retireUnclaimedBenchPost(retireUnclaimedPayload))
		}
	})
}

// BenchmarkRetireUnclaimedRelayWrite measures the explicit relay path
// for the same stub shape: the before leg the lane must not regress.
// The legs compare different routes by necessity (the retire-unclaimed
// path now clones); the relay leg rides POST
// /api/v0/impact/change-surface via foreign-bearer explicit relay on
// its wave-mounted route (change-surface has since terminated natively
// in the impact lane's Wave-3, so the shared-key shape would run
// terminated-vs-terminated); both do exactly one server-side fetch, so
// the 1.50 bound stays a real pathology check.
// retireUnclaimedRelayPost builds the re-pointed relay leg shared by
// the benchmark and the StaysRelayed guard: foreign-bearer POST
// change-surface on its wave-mounted route. Both must use this
// constructor so deleting the Bearer [REDACTED] breaks the guard instead of
// silently restoring terminated-vs-terminated.
func retireUnclaimedRelayPost(bearer string) *http.Request {
	post := authedRequest(http.MethodPost, "/api/v0/impact/change-surface")
	post.Body = io.NopCloser(strings.NewReader(`{"query":"q"}`))
	post.ContentLength = int64(len(`{"query":"q"}`))
	post.Header.Set("Authorization", "Bearer "+bearer)
	return post
}

func BenchmarkRetireUnclaimedRelayWrite(b *testing.B) {
	_, srv := retireUnclaimedBenchServer(b, `{"count":0,"items":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, retireUnclaimedRelayPost("foreign"))
		}
	})
}
