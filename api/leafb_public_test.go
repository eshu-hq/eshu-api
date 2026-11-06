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

// D2 Leaf B wave-public carve-out proof (Refs eshu-hq/eshu#9669, part
// of #6707). The four routes core serves pre-auth register outside any
// gate so dest matches core's public posture byte-for-byte. Byte
// identity of the doc-UI shells with the pinned core spec.go consts
// rides scripts/check-parity.sh (line-verbatim subset gate); these
// tests pin the serving contract around those bytes.

// TestLeafBPublicDocUI pins the doc-UI carve-out: the shells serve
// publicly with core's headers and bytes — no credential, no core
// fetch (unconfigured origin still 200), trailing-slash spelling
// included, v1 alias without deprecation headers.
func TestLeafBPublicDocUI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path  string
		shell string
	}{
		{"/api/v0/docs", leafbSwaggerUIShell},
		{"/api/v0/redoc", leafbReDocShell},
	}
	for _, tc := range cases {
		for _, origin := range []string{"http://core.invalid", ""} {
			for _, target := range []string{tc.path, tc.path + "/"} {
				srv := NewServer(origin, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				req := httptest.NewRequest(http.MethodGet, target, nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Errorf("%s origin %q: status = %d, want 200", target, origin, rec.Code)
					continue
				}
				if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
					t.Errorf("%s origin %q: Content-Type = %q, want the shell shape", target, origin, got)
				}
				body, _ := io.ReadAll(rec.Result().Body)
				if string(body) != tc.shell {
					t.Errorf("%s origin %q: body differs from the ported shell const", target, origin)
				}
			}
		}
	}

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("v1 docs status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("v1 docs Deprecation = %q, want empty", got)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != leafbSwaggerUIShell {
		t.Errorf("v1 docs body differs from the shell const")
	}
}

// TestLeafBPublicDataReads pins the login-picker carve-out: the
// public sign-in-policy and auth-providers reads serve every caller
// (credential or none) from the core on the native lane — the core
// Cache-Control posture relays untouched — and fail closed with the
// permanent local 503 only when no core origin is configured.
func TestLeafBPublicDataReads(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/auth/sign-in-policy?tenant_id=tenant_a",
		"/api/v0/auth/providers?tenant_id=tenant_a",
	} {
		core := &stubCore{
			status: http.StatusOK,
			body:   `{"require_sso":false}`,
			header: http.Header{"Cache-Control": []string{"public, max-age=30"}},
		}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		tel := NewTelemetry(nil)
		srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
		handler := srv.Handler(DefaultV0Sunset)

		callers := []struct {
			name   string
			header map[string]string
		}{
			{"anonymous", nil},
			{"shared key", map[string]string{"Authorization": "Bearer " + skeletonTestKey}},
			{"foreign bearer", map[string]string{"Authorization": "Bearer scoped-token-abc"}},
			{"session cookie", map[string]string{"Cookie": "eshu_session=sess_123"}},
		}
		for _, caller := range callers {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			for k, v := range caller.header {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("%s %s: status = %d, want the relayed core 200", path, caller.name, rec.Code)
			}
			if got := rec.Header().Get("Cache-Control"); got != "public, max-age=30" {
				t.Errorf("%s %s: Cache-Control = %q, want the core posture relayed", path, caller.name, got)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != core.body {
				t.Errorf("%s %s: body = %q, want the core bytes %q", path, caller.name, body, core.body)
			}
		}
		family := classifyFamily(path)
		if got := tel.snapshot().dispatches[family][viaNative][outcomeCoreOK]; got != uint64(len(callers)) {
			t.Errorf("%s: family %q native core_ok = %d, want every caller terminated", path, family, got)
		}
	}

	closed := NewServer("", skeletonTestKey).Handler(DefaultV0Sunset)
	for _, path := range []string{"/api/v0/auth/sign-in-policy", "/api/v0/auth/providers"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		closed.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s without origin: status = %d, want the permanent local 503", path, rec.Code)
		}
	}
}

// TestLeafBProbesStayGreen pins that the lane leaves the live probes
// alone: /health and /readyz stay public, unauthenticated, and green,
// with the probe shapes the deploy gates read.
func TestLeafBProbesStayGreen(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{"/health", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, rec.Code)
		}
		if core.hits != 0 {
			t.Errorf("GET %s contacted the core (%d hits), want probes core-free", path, core.hits)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	for _, want := range []string{`"status":"ok"`, `"core_read_contracts":"v0"`, `"service":"eshu-api"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("health body lacks %s: %s", want, body)
		}
	}
}
