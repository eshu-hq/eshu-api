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

// TestD8Public_CarveOutServesKeyless pins the C-c1 carve-out: the 4
// public routes serve credential-less callers — no dest 401 — and
// forward once to the same core route. This is the behavior change the
// carve-out exists for: before D8 these paths fell through to the
// generic relay inside RequireAuth and answered dest 401s without ever
// reaching core.
func TestD8Public_CarveOutServesKeyless(t *testing.T) {
	t.Parallel()

	for _, tc := range d8PublicCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			payload := d8ParityPayload(tc.method, tc.path)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(payload))
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Errorf("%s Accept %q: upstream hits = %d, want exactly 1 (keyless carve-out forwards)", tc.name, accept, core.hits)
			}
			if core.lastMethod != tc.method || core.lastPath != tc.path {
				t.Errorf("%s Accept %q: upstream = %s %s, want the same core route", tc.name, accept, core.lastMethod, core.lastPath)
			}
			if core.lastBody != payload {
				t.Errorf("%s Accept %q: upstream body = %q, want the opaque caller bytes", tc.name, accept, core.lastBody)
			}
			if core.lastAuth != "" {
				t.Errorf("%s Accept %q: upstream Authorization = %q, want empty (no shared-key injection on public routes)",
					tc.name, accept, core.lastAuth)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("%s Accept %q: status = %d, want relayed 200", tc.name, accept, rec.Code)
			}
		}
	}
}

// TestD8Public_CarveOutIsExactlyFour pins the carve-out boundary:
// only the four exact paths leave the gate. Near-miss paths fall
// through to the keyed generic relay and keep the standing 401 for
// credential-less callers with zero core contact; the bootstrap POST
// stays gated (its shared-operator gate is core's, judged core-side,
// and dest never admits it keyless).
func TestD8Public_CarveOutIsExactlyFour(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"POST /api/v0/auth/local/logins",
		"POST /api/v0/auth/local/login/extra",
		"POST /api/v0/auth/local/bootstrap",
		"POST /api/v0/auth/local/invitations",
		"GET /api/v0/auth/local/login",
		"GET /api/v0/auth/browser-session",
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

// TestD8Public_AuthedCallerKeyCrossesUntouched pins that a shared-key
// caller on a public route forwards its own key as an ordinary header:
// dest neither strips it nor judges it there — core judges every
// credential it receives.
func TestD8Public_AuthedCallerKeyCrossesUntouched(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodPost, "/api/v0/auth/local/login")
	req.Body = io.NopCloser(strings.NewReader(`{"username":"operator","password":"s3cret-proof"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want 1", core.hits)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization = %q, want the caller key untouched", core.lastAuth)
	}
	if core.lastBody != `{"username":"operator","password":"s3cret-proof"}` {
		t.Errorf("upstream body = %q, want the opaque credential bytes", core.lastBody)
	}
}
