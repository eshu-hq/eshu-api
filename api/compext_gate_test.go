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

// Wave-1b per-family authorizer and wave-gate proof (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#101).
// Scope-honesty label: the dest authorizer covers the shared-key
// (AllScopes) identity only — the single class dest can construct
// today. Scoped tokens, OIDC bearers, and browser sessions relay
// explicitly (core judges;
// TestCompextWaveRelayExplicit... pins that), and the deny branch
// below is unreachable on the live route until the D8 session
// resolver lands. Core re-judges every forwarded call as backstop,
// which is where the live 403s come from.
// The routes stay authed: no public carve-out exists in this lane,
// and keyless callers keep the standing dest 401 with zero core
// contact.
//
// Both legs classify to the shared "other" family (no component
// prefix entry exists in classifyFamily): the authorizer and
// telemetry pin that value until a family entry lands.

// TestCompextAuthorizerSharedKeyAllowDeny pins the family gate: the
// shared-key identity allows, anything else denies. One shared
// authorizer covers both wave routes until D8 resolves per-class
// handling.
func TestCompextAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeCompextSharedKey(true) {
		t.Errorf("authorizeCompextSharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeCompextSharedKey(false) {
		t.Errorf("authorizeCompextSharedKey(false) = allow, want deny (family 403)")
	}
}

// TestCompextAuthorizerCountersPinned pins the authorizer telemetry
// for the wave: a terminated shared-key call files one allow judgment
// on the route family, and the unreachable deny branch stays
// observable. One leg per wave route.
func TestCompextAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	for _, leg := range []struct {
		family string
		method string
		path   string
		body   string
	}{
		{"other", http.MethodGet, "/api/v0/component-extensions", ""},
		{"other", http.MethodGet, "/api/v0/component-extensions/comp-1/diagnostics", ""},
	} {
		t.Run(leg.family+" "+leg.method+" "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, compextAuthedReq(leg.method, leg.path, leg.body))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.authorizer[leg.family][decisionAllow]; got != 1 {
				t.Errorf("authorizer[%s][allow] = %d, want 1", leg.family, got)
			}
			if got := snap.authorizer[leg.family][decisionDeny]; got != 0 {
				t.Errorf("authorizer[%s][deny] = %d, want 0 on the allow path", leg.family, got)
			}
		})
	}
}

// TestCompextDenyBranchShapePinned pins the deny shape for the day
// dest gains a scoped/session resolver: a non-AllScopes identity is
// denied with the documented family 403 in both negotiated shapes and
// the core is never reached — for every wave route.
func TestCompextDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, leg := range compextRedLegs {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.method + " " + leg.path + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := compextAuthedReq(leg.method, leg.path, leg.body)
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				srv.serveCompextTerminatedAs(rec, req, false)

				res := rec.Result()
				if res.StatusCode != http.StatusForbidden {
					t.Fatalf("deny status = %d, want the documented 403", res.StatusCode)
				}
				body, _ := io.ReadAll(res.Body)
				if !strings.Contains(string(body), compextFamilyDenyMessage) {
					t.Errorf("body = %q, want it to carry the documented deny text", body)
				}
			})
		}
	}
}

// TestCompextGateCredentialShapes pins the wave gate: a valid
// shared key terminates natively; a foreign bearer or a
// session-cookie-shaped Cookie relays explicitly on the relay lane;
// a credential-less caller or a junk-cookie carrier gets the standing
// 401 with zero core contact; an empty configured key fails closed.
func TestCompextGateCredentialShapes(t *testing.T) {
	t.Parallel()

	newLane := func(t *testing.T, key string) (*stubCore, *httptest.Server, *Server, *Telemetry) {
		t.Helper()
		core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		t.Cleanup(upstream.Close)
		tel := NewTelemetry(nil)
		return core, upstream, NewServer(upstream.URL, key).WithTelemetry(tel), tel
	}
	target := "/api/v0/component-extensions"
	diagnostics := "/api/v0/component-extensions/comp-1/diagnostics"

	t.Run("foreign bearer relays explicitly", func(t *testing.T) {
		t.Parallel()
		core, _, srv, tel := newLane(t, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("Authorization", "Bearer [REDACTED]")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want core's 200 on the relay lane", rec.Code)
		}
		if core.hits != 1 {
			t.Fatalf("core hits = %d, want 1 (explicit relay reaches core)", core.hits)
		}
		snap := tel.snapshot()
		if got := snap.dispatches["other"][viaRelay][outcomeCoreOK]; got != 1 {
			t.Errorf("other relay core_ok = %d, want 1", got)
		}
	})

	t.Run("diagnostics foreign bearer relays explicitly", func(t *testing.T) {
		t.Parallel()
		core, _, srv, tel := newLane(t, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)
		req := httptest.NewRequest(http.MethodGet, diagnostics, nil)
		req.Header.Set("Authorization", "Bearer [REDACTED]")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want core's 200 on the relay lane", rec.Code)
		}
		if core.hits != 1 {
			t.Fatalf("core hits = %d, want 1 (explicit relay reaches core)", core.hits)
		}
		snap := tel.snapshot()
		if got := snap.dispatches["other"][viaRelay][outcomeCoreOK]; got != 1 {
			t.Errorf("other relay core_ok = %d, want 1", got)
		}
	})

	t.Run("session cookie relays explicitly", func(t *testing.T) {
		t.Parallel()
		core, _, srv, tel := newLane(t, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.AddCookie(&http.Cookie{Name: "eshu_session", Value: "browser-1"})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want core's 200 on the relay lane", rec.Code)
		}
		if core.hits != 1 {
			t.Fatalf("core hits = %d, want 1 (explicit relay reaches core)", core.hits)
		}
		snap := tel.snapshot()
		if got := snap.dispatches["other"][viaRelay][outcomeCoreOK]; got != 1 {
			t.Errorf("other relay core_ok = %d, want 1", got)
		}
	})

	t.Run("credential-less keeps 401 with zero core contact", func(t *testing.T) {
		t.Parallel()
		core, _, srv, _ := newLane(t, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
		if core.hits != 0 {
			t.Errorf("core hits = %d, want 0", core.hits)
		}
	})

	t.Run("junk cookie keeps 401 with zero core contact", func(t *testing.T) {
		t.Parallel()
		core, _, srv, _ := newLane(t, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.AddCookie(&http.Cookie{Name: "junk", Value: "not-a-session"})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
		if core.hits != 0 {
			t.Errorf("core hits = %d, want 0", core.hits)
		}
	})

	t.Run("empty configured key fails closed", func(t *testing.T) {
		t.Parallel()
		core, _, srv, _ := newLane(t, "")
		handler := srv.Handler(DefaultV0Sunset)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, compextAuthedReq(http.MethodGet, target, ""))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
		if core.hits != 0 {
			t.Errorf("core hits = %d, want 0", core.hits)
		}
	})

	t.Run("no core origin files permanent local 503", func(t *testing.T) {
		t.Parallel()
		srv := NewServer("", skeletonTestKey)
		for _, via := range []string{viaNative, viaRelay} {
			req := compextAuthedReq(http.MethodGet, target, "")
			rec := httptest.NewRecorder()
			if via == viaNative {
				srv.serveCompextTerminatedAs(rec, req, true)
			} else {
				srv.compextRelayExplicit(rec, req)
			}
			res := rec.Result()
			if res.StatusCode != http.StatusServiceUnavailable {
				t.Errorf("via %s status = %d, want 503", via, res.StatusCode)
			}
			if res.Header.Get("Retry-After") != "" {
				t.Errorf("via %s Retry-After = %q, want empty (never Retry-After)", via, res.Header.Get("Retry-After"))
			}
		}
	})
}
