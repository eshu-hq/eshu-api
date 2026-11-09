// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestGitHubTermNative_CredentialHygiene pins that dest never handles
// the credential material: the state/code query values, the start
// IDs, and the return_to value cross byte-exact to the core while no
// query value, RequestURI, Location, or Set-Cookie value reaches dest
// logs, metrics, or spans. The secrets below are canaries: any
// dest-side parse, log, or telemetry of them fails this test. Dest
// reads no credential values on this lane — the port fetch forwards
// the method, path, raw query, and headers opaquely — so the only
// dest-side sighting would be a leak.
func TestGitHubTermNative_CredentialHygiene(t *testing.T) {
	t.Parallel()

	const secretState = "st-canary-9f2c4e"
	const secretCode = "cd-canary-71ab08"

	core := &stubCore{
		status: http.StatusSeeOther,
		header: http.Header{
			"Location":   []string{"/console?from=canary-loc"},
			"Set-Cookie": []string{"__Host-eshu_session=canary-session; Path=/; HttpOnly; Secure; SameSite=Strict"},
		},
		body: ``,
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	var logs bytes.Buffer
	tel := NewTelemetry(slog.New(slog.NewJSONHandler(&logs, nil)))
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/github/callback?state="+secretState+"&code="+secretCode, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1", core.hits)
	}
	if core.lastQuery != "state="+secretState+"&code="+secretCode {
		t.Errorf("upstream query = %q, want the opaque caller query", core.lastQuery)
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want relayed 303", rec.Code)
	}

	logged := logs.String()
	for _, leaked := range []string{secretState, secretCode, "canary-session", "canary-loc", "RequestURI", "request_uri", "Location", "Set-Cookie", "set-cookie"} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: credential material must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	metricsBody, _ := io.ReadAll(metricsRec.Result().Body)
	for _, leaked := range []string{secretState, secretCode} {
		if strings.Contains(string(metricsBody), leaked) {
			t.Errorf("/metrics contains %q: credential material must never reach metrics", leaked)
		}
	}
}

// TestGitHubTerm_LaneFileHygiene is the credential-hygiene grep proof:
// the lane file carries no query parser, no credential literal, no
// request-URI sighting, no logger/tracer reference, and no store,
// table, or serialization primitive. A parse, log line, handle, or
// lock added to api/github_terminate.go fails here until it is
// removed — the opaque-bytes and stores-stay-core gates are enforced
// by this test, not by review alone.
func TestGitHubTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("github_terminate.go")
	if err != nil {
		t.Fatalf("read github_terminate.go: %v", err)
	}
	body := string(raw)
	for _, forbidden := range []string{
		".Query(",
		"ParseForm(",
		"FormValue(",
		".RequestURI",
		`"Location"`,
		`"Set-Cookie"`,
		`"state"`,
		`"code"`,
		"slog.",
		"tracing.",
		"span.",
		"database/sql",
		"sql.DB",
		"sql.Open",
		"Store",
		"sync.",
		"Mutex",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("github_terminate.go contains %q: lane must stay opaque, log-free, store-free, and lock-free", forbidden)
		}
	}
}

// TestGitHubTerm_KeylessCarriesNoCredential pins that the keyless
// terminated fetch invents no Authorization: the upstream sees an
// empty Authorization, so the dest shared key never crosses on these
// public routes.
func TestGitHubTerm_KeylessCarriesNoCredential(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://github.com/login/oauth/authorize"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/github/login?provider_config_id=github-dev", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want 1", core.hits)
	}
	if core.lastAuth != "" {
		t.Errorf("upstream Authorization = %q, want empty (no shared-key injection on public routes)", core.lastAuth)
	}
}
