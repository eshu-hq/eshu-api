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

// TestD9SAMLNative_CredentialHygiene pins that dest never handles the
// credential material: the provider path value, the return_to query
// value, and the ACS form fields (RelayState, SAMLResponse) relay
// byte-exact to the core while no path value, query value, form
// value, RequestURI, Location, or Set-Cookie value reaches dest logs,
// metrics, or spans. The secrets below are canaries: any dest-side
// parse, log, or telemetry of them fails this test. Dest reads no
// credential values on this lane — forwardToCore forwards the method,
// path, query, and body opaquely — so the only dest-side sighting
// would be a leak.
func TestD9SAMLNative_CredentialHygiene(t *testing.T) {
	t.Parallel()

	const secretRelay = "relay-canary-9f2c4e"
	const secretResponse = "response-canary-71ab08"

	core := &stubCore{
		status: http.StatusSeeOther,
		header: http.Header{
			"Location":   []string{"/console"},
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

	form := "RelayState=" + secretRelay + "&SAMLResponse=" + secretResponse
	req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1", core.hits)
	}
	if core.lastBody != form {
		t.Errorf("upstream body = %q, want the opaque caller form relayed byte-exact", core.lastBody)
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want relayed 303", rec.Code)
	}

	logged := logs.String()
	for _, leaked := range []string{secretRelay, secretResponse, "canary-session", "RequestURI", "request_uri", "RelayState", "SAMLResponse", "Location", "Set-Cookie", "set-cookie"} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: credential material must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	metricsBody, _ := io.ReadAll(metricsRec.Result().Body)
	for _, leaked := range []string{secretRelay, secretResponse} {
		if strings.Contains(string(metricsBody), leaked) {
			t.Errorf("/metrics contains %q: credential material must never reach metrics", leaked)
		}
	}
}

// TestD9SAMLNative_LargeFormBodyStaysOpaque pins the POST ACS opacity
// bound: a near-1MiB form body and an over-1MiB form body both forward
// byte-intact with exactly one upstream attempt each, and core's own
// answer (the stub stands in for core's MaxBytesReader 400 on the
// oversize shape) relays unmodified. Dest applies no body cap of its
// own on this lane — the only cap lives core-side — and parses
// nothing: the test would fail on any truncation, second attempt, or
// dest-side refusal.
func TestD9SAMLNative_LargeFormBodyStaysOpaque(t *testing.T) {
	t.Parallel()

	near := "RelayState=" + strings.Repeat("r", 512*1024) + "&SAMLResponse=" + strings.Repeat("s", 512*1024-64*1024)
	over := "RelayState=" + strings.Repeat("r", 640*1024) + "&SAMLResponse=" + strings.Repeat("s", 640*1024)
	for _, tc := range []struct {
		name   string
		form   string
		status int
		body   string
	}{
		{"near-1MiB relays 201", near, http.StatusCreated, `{"csrf_token":"opaque-csrf"}`},
		{"over-1MiB relays core 400", over, http.StatusBadRequest, `{"error":"Bad Request","detail":"invalid saml response form"}`},
	} {
		core := &stubCore{status: tc.status, header: nil, body: tc.body}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(tc.form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 {
			t.Errorf("%s: upstream hits = %d, want exactly 1", tc.name, core.hits)
		}
		if core.lastBody != tc.form {
			t.Errorf("%s: upstream body differs from caller form: len = %d, want %d (byte-intact)",
				tc.name, len(core.lastBody), len(tc.form))
		}
		if rec.Code != tc.status {
			t.Errorf("%s: status = %d, want relayed %d", tc.name, rec.Code, tc.status)
		}
		respBody, _ := io.ReadAll(rec.Result().Body)
		if string(respBody) != tc.body {
			t.Errorf("%s: body = %q, want the core bytes %q", tc.name, respBody, tc.body)
		}
	}
}

// TestD9SAMLNative_NoDestSideFormReads pins the POST ACS opacity at
// the source level: the lane file must contain no form or body read —
// no ParseForm, no FormValue, no PostForm, no MultipartForm, no
// body-ReadAll, no dest-side cap. The form crosses only inside the
// shared streaming forward. If a future edit parses the form
// dest-side, this test fails closed.
func TestD9SAMLNative_NoDestSideFormReads(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("d9saml.go")
	if err != nil {
		t.Fatalf("read d9saml.go: %v", err)
	}
	for _, banned := range []string{"ParseForm", "FormValue", "PostForm", "MultipartForm", "ReadAll", "MaxBytesReader", "MultipartReader"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("d9saml.go contains %q: the ACS form must cross opaquely, never parsed dest-side", banned)
		}
	}
}

// TestD9SAMLNative_NoDestSideClientSymbols pins decision 3 at the
// source level: the lane file must name no dest-side client type,
// interface, or import — no store, no provider config, no verifier,
// no keyring, no client library. HTTP bytes cross via the existing
// shared streaming forward only. If a future edit grows a dest-side
// client, this test fails closed (and scripts/check-parity.sh gates
// the same list over the lane file).
func TestD9SAMLNative_NoDestSideClientSymbols(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("d9saml.go")
	if err != nil {
		t.Fatalf("read d9saml.go: %v", err)
	}
	for _, banned := range []string{
		"SAMLStore", "SAMLProviderConfig", "ServiceProviderConfig",
		"samlauth", "secretcrypto", "keyring",
	} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("d9saml.go contains %q: no dest-side client type, import, or library on this lane", banned)
		}
	}
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, `"`) && strings.Contains(strings.ToLower(trimmed), "saml") {
			t.Errorf("d9saml.go imports %s: no client-library import on this lane (net/http only)", trimmed)
		}
	}
}
