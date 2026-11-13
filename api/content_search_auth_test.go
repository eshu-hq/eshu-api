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

// W4 Content search per-family authorizer and wave-gate proof (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#111).
// Scope-honesty label: the dest authorizer covers the shared-key
// (AllScopes) identity only — the single class dest can construct
// today. Scoped tokens, OIDC bearers, and browser sessions relay
// explicitly (core judges;
// TestContentSearchWaveRelayExplicit... pins that), and the deny
// branch below is unreachable on the live route until the D8 session
// resolver lands. Core re-judges every forwarded call as backstop,
// which is where the live 403s come from. The routes stay authed: no
// public carve-out exists in this lane, and keyless callers keep the
// standing dest 401 with zero core contact.

// TestContentSearchAuthorizerSharedKeyAllowDeny pins the family gate:
// the shared-key identity allows, anything else denies. One shared
// authorizer covers the four search wave routes until D8 resolves
// per-class handling.
func TestContentSearchAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeContentSearchSharedKey(true) {
		t.Errorf("authorizeContentSearchSharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeContentSearchSharedKey(false) {
		t.Errorf("authorizeContentSearchSharedKey(false) = allow, want deny (family 403)")
	}
}

// TestContentSearchAuthorizerCountersPinned pins the authorizer
// telemetry for the wave: terminated shared-key calls file one allow
// judgment each on the content family, and the unreachable deny branch
// stays observable.
func TestContentSearchAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range contentSearchConcretePaths {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, contentAuthedPost(path, contentSearchParityBodies[path][0], "application/json"))
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s: status = %d, want 200", path, rec.Code)
		}
	}
	snap := tel.snapshot()
	if got := snap.authorizer["content"][decisionAllow]; got != 4 {
		t.Errorf("authorizer[content][allow] = %d, want 4 (one per search leg)", got)
	}
	if got := snap.authorizer["content"][decisionDeny]; got != 0 {
		t.Errorf("authorizer[content][deny] = %d, want 0 on the allow path", got)
	}
}

// TestContentSearchDenyBranchShapePinned pins the deny shape for the
// day dest gains a scoped/session resolver: a non-AllScopes identity
// is denied with the documented family 403 in both negotiated shapes
// and the core is never reached.
func TestContentSearchDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	for _, path := range contentSearchConcretePaths {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := "POST " + path + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)

				req := contentAuthedPost(path, contentSearchParityBodies[path][0], accept)
				rec := httptest.NewRecorder()
				srv.serveContentSearchTerminatedAs(rec, req, false)

				res := rec.Result()
				if res.StatusCode != http.StatusForbidden {
					t.Fatalf("status = %d, want the documented 403", res.StatusCode)
				}
				body, _ := io.ReadAll(res.Body)
				if !strings.Contains(string(body), contentSearchFamilyDenyMessage) {
					t.Errorf("body = %q, want it to carry the documented deny text", body)
				}
				if core.hits != 0 {
					t.Errorf("upstream hits = %d, want 0 (deny never reaches the core)", core.hits)
				}
			})
		}
	}
}

// TestContentSearchWaveRelayExplicitForUnresolvableIdentities pins the
// W4 ruling's never-invent rule for the wave: each search POST carrying
// a credential class dest cannot resolve (a scoped token, an OIDC
// bearer, a browser session cookie) must relay explicitly to the core —
// the core judges, core bytes come back untouched — and must never
// answer a dest-invented 401, 403, 404, or 500. Every leg also files
// its dispatch on the relay lane.
func TestContentSearchWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	for _, path := range contentSearchConcretePaths {
		for _, tc := range []struct {
			name   string
			header map[string]string
		}{
			{"scoped bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
			{"oidc bearer", map[string]string{"Authorization": "Bearer [REDACTED]"}},
			{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
			{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
		} {
			t.Run("POST "+path+" "+tc.name, func(t *testing.T) {
				t.Parallel()

				body := contentSearchParityBodies[path][0]
				core := &stubCore{
					status: http.StatusOK,
					body:   `{"results":[]}`,
				}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
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
				respBody, _ := io.ReadAll(rec.Result().Body)
				if string(respBody) != core.body {
					t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, respBody, core.body)
				}
				if core.lastBody != body {
					t.Errorf("%s: upstream body = %q, want the search request %q forwarded intact", tc.name, core.lastBody, body)
				}
				snap := tel.snapshot()
				lanes := snap.dispatches["content"]
				if lanes[viaRelay] == nil {
					t.Errorf("%s: family %q dispatches = %v, want a relay lane row (never invented)", tc.name, "content", lanes)
				}
			})
		}
	}
}

// TestContentSearchWaveGateEmptyKey401 pins the fail-closed gate: with
// no configured key every wave call answers the standing dest 401 with
// zero core contact, for the bare and slash spellings.
func TestContentSearchWaveGateEmptyKey401(t *testing.T) {
	t.Parallel()

	for _, path := range contentSearchConcretePaths {
		for _, target := range []string{path, path + "/"} {
			t.Run("POST "+target, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, "")
				handler := srv.Handler(DefaultV0Sunset)

				req := contentAuthedPost(target, contentSearchParityBodies[path][0], "application/json")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusUnauthorized {
					t.Errorf("status = %d, want the standing dest 401", rec.Code)
				}
				if core.hits != 0 {
					t.Errorf("upstream hits = %d, want 0 (gate denies with zero core contact)", core.hits)
				}
			})
		}
	}
}

// TestContentSearchWaveJunkCookieStays401 pins the D2 follow-up lesson
// for the wave: a Cookie header with no session-cookie member is not a
// credential and answers the standing dest 401 with zero core contact.
func TestContentSearchWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	for _, path := range contentSearchConcretePaths {
		t.Run("POST "+path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(contentSearchParityBodies[path][0]))
			req.Header.Set("Cookie", "theme=dark")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401 for a junk cookie", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (junk cookie never reaches the core)", core.hits)
			}
		})
	}
}

// TestContentSearchWaveCredentiallessStays401 pins that a caller with
// no credential at all answers the standing dest 401 with zero core
// contact on every wave route.
func TestContentSearchWaveCredentiallessStays401(t *testing.T) {
	t.Parallel()

	for _, path := range contentSearchConcretePaths {
		t.Run("POST "+path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(contentSearchParityBodies[path][0]))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (credential-less callers never reach the core)", core.hits)
			}
		})
	}
}
