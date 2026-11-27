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

// Collector-failure acknowledgement tier C-a clone-latency evidence
// (Refs eshu-hq/eshu#9669, part of #6707). The tier-C clone does the
// same single server-side fetch as the generic relay plus the explicit
// route match and the family authorizer; this benchmark bounds that
// overhead so scripts/measure-terminalfailure-latency.sh can gate it
// in CI. The stub core answers from memory — the numbers isolate
// dest-side cost, not core or network time. Absolute deployment numbers
// ride the pinned-core-image leg at flip prep (D11).

// terminalFailureBenchServer builds a Handler over a stub core
// answering body for every route, so clone and relay benchmarks share
// the same upstream shape.
func terminalFailureBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// terminalFailureBenchPost builds one shared-key acknowledge POST with
// the representative operator body, rebuilt per iteration since the
// server consumes it.
func terminalFailureBenchPost(body string) *http.Request {
	post := authedRequest(http.MethodPost, "/api/v0/admin/collector-failures/acknowledge")
	post.Body = io.NopCloser(strings.NewReader(body))
	post.ContentLength = int64(len(body))
	return post
}

// BenchmarkTerminalFailureNativeWrite measures the clone path for a
// shared-key acknowledge POST through the full production chain.
func BenchmarkTerminalFailureNativeWrite(b *testing.B) {
	_, srv := terminalFailureBenchServer(b, `{"status":"acknowledged"}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("acknowledge", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, terminalFailureBenchPost(ackPayload))
		}
	})
}

// BenchmarkTerminalFailureRelayWrite measures the explicit relay path
// for the same stub shape: the before leg the lane must not regress.
// The legs compare different routes by necessity (the acknowledge path
// now clones); the relay leg rides POST
// /api/v0/impact/change-surface via foreign-bearer explicit relay on
// its wave-mounted route (change-surface has since terminated natively
// in the impact lane's Wave-3, so the shared-key shape would run
// terminated-vs-terminated); both do exactly one server-side fetch, so
// the 1.50 bound stays a real pathology check.
// terminalFailureRelayPost builds the re-pointed relay leg shared by
// the benchmark and the StaysRelayed guard: foreign-bearer POST
// change-surface on its wave-mounted route. Both must use this
// constructor so deleting the Bearer [REDACTED] breaks the guard instead of
// silently restoring terminated-vs-terminated.
func terminalFailureRelayPost(bearer string) *http.Request {
	post := authedRequest(http.MethodPost, "/api/v0/impact/change-surface")
	post.Body = io.NopCloser(strings.NewReader(`{"query":"q"}`))
	post.ContentLength = int64(len(`{"query":"q"}`))
	post.Header.Set("Authorization", "Bearer "+bearer)
	return post
}

func BenchmarkTerminalFailureRelayWrite(b *testing.B) {
	_, srv := terminalFailureBenchServer(b, `{"status":"acknowledged"}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, terminalFailureRelayPost("foreign"))
		}
	})
}
