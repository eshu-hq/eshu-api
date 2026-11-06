// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

// D4 Native-next relay-explicit and credential-shape proof (Refs
// eshu-hq/eshu#9669, part of #6707). The D-wave ruling's never-invent
// rule: a wave-gated route carrying a credential class dest cannot
// resolve (a scoped token, an OIDC bearer, a browser session cookie)
// must relay explicitly to the core — the core judges, core bytes come
// back untouched — and must never answer a dest-invented 401, 403,
// 404, or 500. Junk cookies stay dest-401 with zero core contact.

// TestD4SlashSkipPinned pins the trailing-slash skip set: exactly the
// four literal routes whose slash subtree would panic against the
// wildcard sibling under Go 1.22+ ServeMux overlap rules. A wave
// change that adds or removes a literal/wildcard sibling pair must move
// this set with it.
func TestD4SlashSkipPinned(t *testing.T) {
	t.Parallel()

	want := []string{
		"GET /api/v0/documentation/findings/count",
		"GET /api/v0/documentation/findings/inventory",
		"GET /api/v0/repositories/by-language",
		"GET /api/v0/repositories/language-inventory",
	}
	if len(d4SlashSkip) != len(want) {
		t.Fatalf("d4SlashSkip holds %d entries, want %d", len(d4SlashSkip), len(want))
	}
	got := make([]string, 0, len(d4SlashSkip))
	for pattern := range d4SlashSkip {
		got = append(got, pattern)
	}
	sort.Strings(got)
	sort.Strings(want)
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("d4SlashSkip differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestD4WaveRelayExplicitForUnresolvableIdentities pins the never-invent
// rule for the whole wave: every D4 route carrying a foreign credential
// class relays explicitly on the relay lane with core bytes untouched.
func TestD4WaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, entry := range d4Wave {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()

			for _, tc := range []struct {
				name   string
				header map[string]string
			}{
				{"scoped bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
				{"oidc bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
				{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
				{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
			} {
				core := &stubCore{
					status: http.StatusOK,
					body:   `{"data":[],"error":null,"truth":null}`,
				}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				key := entry.Method + " " + entry.Path
				req := httptest.NewRequest(entry.Method, d4ConcretePaths[key], nil)
				for k, v := range tc.header {
					req.Header.Set(k, v)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Errorf("%s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, core.hits)
				}
				if rec.Code != http.StatusOK {
					t.Errorf("%s: status = %d, want the relayed core 200 (never 401/403/404/500-invented)", tc.name, rec.Code)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != core.body {
					t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, body, core.body)
				}
				family := classifyFamily(d4ConcretePaths[key])
				if got := tel.snapshot().dispatches[family][viaRelay][outcomeCoreOK]; got != 1 {
					t.Errorf("%s: family %q relay core_ok = %d, want 1", tc.name, family, got)
				}
			}
		})
	}
}

// TestD4WaveJunkCookieStays401 pins the D2 follow-up cookie lesson:
// only session-cookie-shaped credentials relay. A Cookie header with
// no eshu_session (or __Host-eshu_session) member is not a browser
// session — it stays dest-401 with zero core contact, exactly like a
// credential-less caller, in both negotiated shapes.
func TestD4WaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, entry := range d4Wave {
		for _, cookie := range []string{
			"theme=dark",
			"unrelated=ignore; other=1",
			"eshu_session_stale=",
		} {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				key := entry.Method + " " + entry.Path
				req := httptest.NewRequest(entry.Method, d4ConcretePaths[key], nil)
				req.Header.Set("Accept", accept)
				req.Header.Set("Cookie", cookie)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusUnauthorized {
					t.Errorf("%s %s cookie %q Accept %q: status = %d, want 401 (junk cookies never relay)",
						entry.Method, entry.Path, cookie, accept, rec.Code)
				}
				if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
					t.Errorf("%s %s cookie %q Accept %q: WWW-Authenticate = %q, want bare Bearer",
						entry.Method, entry.Path, cookie, accept, got)
				}
			}
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (junk cookies never reach the core)", core.hits)
	}
}

// TestD4ConflictLiterals_TrailingSlashStaysNative pins the d4SlashSkip
// serving: the four literals whose slash subtree would panic against
// the wildcard sibling skip the duplicate registration, and the slash
// spellings still terminate natively under the parent list-subtree
// registration (findings/, repositories/) — the never-split rule holds,
// the core sees the slash spelling, and the mux never panics at build.
func TestD4ConflictLiterals_TrailingSlashStaysNative(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	targets := []struct{ method, path string }{
		{http.MethodGet, "/api/v0/documentation/findings/count/"},
		{http.MethodGet, "/api/v0/documentation/findings/inventory/"},
		{http.MethodGet, "/api/v0/repositories/by-language/"},
		{http.MethodGet, "/api/v0/repositories/language-inventory/"},
	}
	for _, tc := range targets {
		req := authedRequest(tc.method, tc.path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s %s: status = %d, want 200 from the relayed core", tc.method, tc.path, rec.Code)
		}
		if core.lastPath != tc.path {
			t.Errorf("%s %s: upstream path = %q, want the slash spelling relayed", tc.method, tc.path, core.lastPath)
		}
	}
	snap := tel.snapshot()
	for _, family := range []string{"documentation", "repositories"} {
		if got := snap.dispatches[family][viaNative][outcomeCoreOK]; got < 2 {
			t.Errorf("family %q native core_ok = %d, want >= 2 (both slash spellings on the native lane)", family, got)
		}
	}
}
