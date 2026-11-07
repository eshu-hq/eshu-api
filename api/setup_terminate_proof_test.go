// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// D10 setup-state termination boundary proofs (Refs eshu-hq/eshu#9669,
// part of #6707): the public carve-out boundary (exactly four routes,
// no fifth), the telemetry extension without renames, and the
// MFA-issued session end-to-end usability through the D8 gate. The
// transport proofs live in setup_terminate_test.go.

// TestSetupTerminated_CarveOutIsExactlyFour pins the public boundary
// the lane graduates to D8-grade handling with: near-miss setup
// spellings are not carved out — keyless callers keep the standing
// 401 with zero core contact. No fifth public route.
func TestSetupTerminated_CarveOutIsExactlyFour(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	// The twin subtree (setup-state/...) still terminates on the
	// canonical fetch — see TestSetupTerminated_SlashTwinSubtree —
	// so only spellings outside both twin subtrees belong here.
	for _, target := range []string{
		"GET /api/v0/auth/setup-stat",
		"POST /api/v0/auth/setup-states",
		"POST /api/v0/auth/setup/claims",
		"GET /api/v0/auth/setup/claim",
		"POST /api/v0/auth/setup-state",
	} {
		method, path, _ := strings.Cut(target, " ")
		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want the standing 401 (not carved out)", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (gate denials never dispatch)", core.hits)
	}
}

// TestSetupTerminated_TelemetryExtended pins the telemetry extension
// without renames: the terminated GET files the setup family on the
// existing native lane with the closed outcome vocabulary, records
// core latency for the port fetch, emits one closed-label log line
// with no credential bytes, and files no authorizer judgment (the
// route is public — there is no gate to judge).
func TestSetupTerminated_TelemetryExtended(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"needs_setup":true,"bootstrap_mode":"generated"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	var logs strings.Builder
	srv := NewServer(upstream.URL, skeletonTestKey)
	tel := NewTelemetry(slog.New(slog.NewJSONHandler(&logs, nil)))
	srv.WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want terminated 200", rec.Code)
	}

	snap := tel.snapshot()
	if snap.requests["setup"]["2xx"] != 1 {
		t.Errorf("requests rows = %v, want one setup/2xx", snap.requests["setup"])
	}
	if snap.dispatches["setup"]["native"]["core_ok"] != 1 {
		t.Errorf("dispatch rows = %v, want one setup/native/core_ok (existing lane, extended transport)",
			snap.dispatches["setup"])
	}
	if got := len(snap.coreLatencies["setup"]["native"].buckets); got == 0 {
		t.Errorf("no setup/native core-latency histogram, want the port fetch timed")
	}
	if _, ok := snap.authorizer["setup"]; ok {
		t.Errorf("authorizer rows = %v, want none (public route files no judgment)", snap.authorizer["setup"])
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want 1 (one closed-label line per request)", len(lines))
	}
	for _, secret := range []string{skeletonTestKey, "bootstrap-p"} {
		if strings.Contains(lines[0], secret) {
			t.Errorf("structured log contains %q, want closed labels only", secret)
		}
	}
}

// TestSetupMFAIssuedSessionUsableThroughD8 pins condition 5 by proof:
// the session cookie the MFA step issues (the real core cookie name
// the D8 gate recognizes) crosses dest on a D8 session route end to
// end — dest relays the cookie carrier to core instead of denying it.
// A junk cookie still keeps the standing 401 with zero core contact.
func TestSetupMFAIssuedSessionUsableThroughD8(t *testing.T) {
	t.Parallel()

	const sessionCookie = "eshu_session=opaque-session-proof"
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/api/v0/auth/setup/mfa" {
			w.Header().Set("Set-Cookie", sessionCookie+"; Path=/; HttpOnly")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"status":"completed","recovery_codes":["r1","r2"],"auth":{}}`)
			return
		}
		if r.URL.Path == "/api/v0/auth/sessions" {
			if r.Header.Get("Cookie") != sessionCookie {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"sessions":[]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	mfaReq := httptest.NewRequest(http.MethodPost, "/api/v0/auth/setup/mfa",
		strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`))
	mfaRec := httptest.NewRecorder()
	handler.ServeHTTP(mfaRec, mfaReq)
	if mfaRec.Code != http.StatusOK {
		t.Fatalf("MFA status = %d, want relayed 200", mfaRec.Code)
	}
	issued := mfaRec.Result().Header.Get("Set-Cookie")
	if !strings.HasPrefix(issued, sessionCookie) {
		t.Fatalf("MFA Set-Cookie = %q, want the relayed session cookie", issued)
	}

	sessionsReq := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sessions", nil)
	sessionsReq.Header.Set("Cookie", sessionCookie)
	sessionsRec := httptest.NewRecorder()
	handler.ServeHTTP(sessionsRec, sessionsReq)
	if sessionsRec.Code != http.StatusOK {
		t.Errorf("session-cookie sessions status = %d, want relayed 200 (usable through D8)", sessionsRec.Code)
	}
	gotBody, _ := io.ReadAll(sessionsRec.Result().Body)
	if string(gotBody) != `{"sessions":[]}` {
		t.Errorf("session-cookie sessions body = %q, want the core bytes", gotBody)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("upstream hits = %d, want 2 (MFA issue plus session-cookie relay)", got)
	}

	junkReq := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sessions", nil)
	junkReq.Header.Set("Cookie", "junk=value")
	junkRec := httptest.NewRecorder()
	handler.ServeHTTP(junkRec, junkReq)
	if junkRec.Code != http.StatusUnauthorized {
		t.Errorf("junk-cookie sessions status = %d, want the standing dest 401", junkRec.Code)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("upstream hits = %d, want still 2 (junk never dispatches)", got)
	}
}
