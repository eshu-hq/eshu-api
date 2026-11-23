// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// D8 session-resolver latency evidence (Refs eshu-hq/eshu#9669, part of
// #6707). The tier-C clone does the same single server-side fetch as
// the generic relay plus the explicit route match, the credential-class
// gate, and the family authorizer; these benchmarks bound that overhead
// per representative route so scripts/measure-d8-latency.sh can gate it
// in CI. The stub core answers from memory — the numbers isolate
// dest-side cost, not core or network time. Absolute deployment numbers
// ride the pinned-core-image leg at flip prep (D11).

// d8BenchServer builds a Handler over a stub core answering body for
// every route, so clone and relay benchmarks share the same upstream
// shape.
func d8BenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkD8NativeWrite measures the clone path for a shared-key POST
// through the full production chain: one browser-session create leg
// (the lifecycle write) and one API-token create leg (the token
// write).
func BenchmarkD8NativeWrite(b *testing.B) {
	_, srv := d8BenchServer(b, `{"status":"created"}`)
	handler := srv.Handler(DefaultV0Sunset)

	browserCreate := authedRequest(http.MethodPost, "/api/v0/auth/browser-session")
	tokenCreate := authedRequest(http.MethodPost, "/api/v0/auth/local/api-tokens")

	b.Run("browser-create", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, browserCreate.Clone(browserCreate.Context()))
		}
	})
	b.Run("token-create", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, tokenCreate.Clone(tokenCreate.Context()))
		}
	})
}

// BenchmarkD8PublicWrite measures the public carve-out path for a
// keyless POST through the full production chain: the login leg (the
// credential-bearing write that left RequireAuth).
func BenchmarkD8PublicWrite(b *testing.B) {
	_, srv := d8BenchServer(b, `{"status":"accepted"}`)
	handler := srv.Handler(DefaultV0Sunset)

	login := httptest.NewRequest(http.MethodPost, "/api/v0/auth/local/login", nil)

	b.Run("login", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, login.Clone(login.Context()))
		}
	})
}

// BenchmarkD8NativeRead measures the clone path for a shared-key GET
// through the full production chain: one session-list leg and one
// profile leg.
func BenchmarkD8NativeRead(b *testing.B) {
	_, srv := d8BenchServer(b, `{"sessions":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	sessions := authedRequest(http.MethodGet, "/api/v0/auth/sessions")
	profile := authedRequest(http.MethodGet, "/api/v0/auth/profile")

	b.Run("sessions", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, sessions.Clone(sessions.Context()))
		}
	})
	b.Run("profile", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, profile.Clone(profile.Context()))
		}
	})
}

// BenchmarkD8RelayWrite measures the explicit relay path for the
// same stub shape: POST /api/v0/impact/change-surface rides
// foreign-bearer explicit relay on its wave-mounted route
// (change-surface has since terminated natively in the impact
// lane's Wave-3, so the shared-key shape would run
// terminated-vs-terminated). The legs compare different routes by
// necessity (the D8 paths now clone); both do exactly one
// server-side fetch, so the 1.50 bound stays a real pathology check.
// d8RelayPost and d8RelayGet build the re-pointed relay legs shared by
// the benchmarks and the StaysRelayed guard: foreign-bearer legs on their
// wave-mounted routes. Both must use these constructors so deleting a bearer
// breaks the guard instead of silently restoring terminated-vs-terminated.
func d8RelayPost(bearer string) *http.Request {
	post := authedRequest(http.MethodPost, "/api/v0/impact/change-surface")
	post.Header.Set("Authorization", "Bearer "+bearer)
	return post
}

func d8RelayGet(bearer string) *http.Request {
	get := authedRequest(http.MethodGet, "/api/v0/dependencies")
	get.Header.Set("Authorization", "Bearer "+bearer)
	return get
}

func BenchmarkD8RelayWrite(b *testing.B) {
	_, srv := d8BenchServer(b, `{"status":"accepted"}`)
	handler := srv.Handler(DefaultV0Sunset)

	post := d8RelayPost("foreign")

	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post.Clone(post.Context()))
		}
	})
}

// BenchmarkD8RelayRead measures the explicit relay path for the read
// shape: GET /api/v0/dependencies rides foreign-bearer explicit relay
// on its wave-mounted route (dependencies has since terminated
// natively in W8b-narrow, so the shared-key shape would run
// terminated-vs-terminated). Both legs do exactly one server-side
// fetch, so the 1.50 bound stays a real pathology check.
func BenchmarkD8RelayRead(b *testing.B) {
	_, srv := d8BenchServer(b, `{"dependencies":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	get := d8RelayGet("foreign")

	b.Run("get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, get.Clone(get.Context()))
		}
	})
}
