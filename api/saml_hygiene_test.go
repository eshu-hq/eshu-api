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

// TestSAMLTermNative_CredentialHygiene pins that dest never handles
// the credential material: the provider path value, the return_to
// query value, and the ACS form fields (RelayState, SAMLResponse)
// cross byte-exact to the core while no path value, query value, form
// value, RequestURI, Location, or Set-Cookie value reaches dest logs,
// metrics, or spans. The secrets below are canaries: any dest-side
// parse, log, or telemetry of them fails this test. Dest reads no
// credential values on this lane — the port fetch forwards the
// method, path, raw query, headers, and bounded body opaquely — so the
// only dest-side sighting would be a leak.
func TestSAMLTermNative_CredentialHygiene(t *testing.T) {
	t.Parallel()

	const secretRelay = "relay-canary-9f2c4e"
	const secretResponse = "response-canary-71ab08"
	const secretProvider = "provider-canary-4d1a"

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

	form := "RelayState=" + secretRelay + "&SAMLResponse=" + secretResponse
	req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/"+secretProvider+"/acs?return_to=%2Fconsole", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1", core.hits)
	}
	if core.lastBody != form {
		t.Errorf("upstream body = %q, want the opaque caller form byte-exact", core.lastBody)
	}
	if core.lastPath != "/api/v0/auth/saml/providers/"+secretProvider+"/acs" {
		t.Errorf("upstream path = %q, want the opaque caller path", core.lastPath)
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want relayed 303", rec.Code)
	}

	logged := logs.String()
	for _, leaked := range []string{secretRelay, secretResponse, secretProvider, "canary-session", "canary-loc", "RequestURI", "request_uri", "RelayState", "SAMLResponse", "Location", "Set-Cookie", "set-cookie"} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: credential material must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	metricsBody, _ := io.ReadAll(metricsRec.Result().Body)
	for _, leaked := range []string{secretRelay, secretResponse, secretProvider} {
		if strings.Contains(string(metricsBody), leaked) {
			t.Errorf("/metrics contains %q: credential material must never reach metrics", leaked)
		}
	}
}

// TestSAMLTerm_LaneFileHygiene is the credential-hygiene grep proof:
// the lane file carries no query parser, no form parser, no provider
// extraction, no credential literal, no request-URI sighting, no
// logger/tracer reference, and no store, table, SQL, or serialization
// primitive — and, binding for this lane, no GetSAMLProvider sighting
// (that store method returns the SP key material that never leaves
// core). A parse, log line, handle, or lock added to
// api/saml_terminate.go fails here until it is removed — the
// opaque-bytes and stores-stay-core gates are enforced by this test,
// not by review alone. The bounded ACS body read (ReadAll over a
// LimitReader) is the one deliberate exception: size-bounding is not
// parsing.
func TestSAMLTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("saml_terminate.go")
	if err != nil {
		t.Fatalf("read saml_terminate.go: %v", err)
	}
	body := string(raw)
	for _, forbidden := range []string{
		".Query(",
		"ParseForm(",
		"FormValue(",
		"PostForm(",
		"MultipartForm(",
		"MultipartReader(",
		"MaxBytesReader(",
		"PathValue(",
		".RequestURI",
		"\"Location\"",
		"\"Set-Cookie\"",
		"\"RelayState\"",
		"\"SAMLResponse\"",
		"\"provider_id\"",
		"\"return_to\"",
		"GetSAMLProvider",
		"SAMLStore",
		"SAMLProviderConfig",
		"ServiceProviderConfig",
		"samlauth",
		"secretcrypto",
		"keyring",
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
			t.Errorf("saml_terminate.go contains %q: lane must stay opaque, log-free, store-free, and lock-free", forbidden)
		}
	}
}

// TestSAMLTerm_KeylessCarriesNoCredential pins that the keyless
// terminated fetch invents no Authorization: the upstream sees an
// empty Authorization, so the dest shared key never crosses on these
// public routes.
func TestSAMLTerm_KeylessCarriesNoCredential(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, tc := range []struct {
		method string
		target string
		body   string
	}{
		{http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", ""},
		{http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", ""},
		{http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", "RelayState=opaque& SAMLResponse=opaque"},
	} {
		var req *http.Request
		if tc.body != "" {
			req = httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req = httptest.NewRequest(tc.method, tc.target, nil)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.lastAuth != "" {
			t.Errorf("%s %s: upstream Authorization = %q, want empty (no shared-key injection on public routes)", tc.method, tc.target, core.lastAuth)
		}
	}
	if core.hits != 3 {
		t.Errorf("upstream hits = %d, want 3 (one fetch per leg)", core.hits)
	}
}
