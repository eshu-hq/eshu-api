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

// W7-read lane-file hygiene (Refs eshu-hq/eshu#9669, D13 tracker
// eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#72).

// TestAdminSignInPolicyTerm_LaneFileHygiene is the no-core-import
// grep proof at the source level: the lane file carries no dest-side
// store, port, or table symbol (the pin's store stays core-side), no
// query/form/path parser, no request-URI sighting, no logger/tracer
// reference, and no serialization primitive or lock. A store handle,
// parse, log line, or lock added to
// api/signinpolicy_admin_terminate.go fails here until it is removed
// — the stores-stay-core and opaque-bytes gates are enforced by this
// test, not by review alone. scripts/check-parity.sh gates the same
// list.
func TestAdminSignInPolicyTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("signinpolicy_admin_terminate.go")
	if err != nil {
		t.Fatalf("read signinpolicy_admin_terminate.go: %v", err)
	}
	body := string(raw)
	for _, forbidden := range []string{
		"postgres",
		"SignInPolicyStore",
		"ReadStore",
		"IdentitySubjectStore",
		"database/sql",
		"sql.DB",
		"sql.Open",
		".Query(",
		"ParseForm(",
		"FormValue(",
		"PathValue(",
		".RequestURI",
		`"Location"`,
		`"Set-Cookie"`,
		`"tenant_id"`,
		"slog.",
		"tracing.",
		"span.",
		"json.Marshal",
		"sync.",
		"Mutex",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("signinpolicy_admin_terminate.go contains %q: lane must stay store-free, parse-free, log-free, and lock-free", forbidden)
		}
	}
}

// TestAdminSignInPolicyTerm_CredentialHygiene pins that dest never
// handles the credential material: the shared key crosses byte-exact
// to the core while no Authorization value reaches dest logs,
// metrics, or spans. The canary below fails this test on any
// dest-side parse, log, or telemetry of it. Dest reads no credential
// values on this lane — the forward carries the credential opaquely
// — so the only dest-side sighting would be a leak.
func TestAdminSignInPolicyTerm_CredentialHygiene(t *testing.T) {
	t.Parallel()

	const canaryKey = "canary-key-9f2c4e71ab08"

	core := &stubCore{status: http.StatusOK, body: `{"tenant_id":"tenant_a","require_sso":false}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	var logs bytes.Buffer
	tel := NewTelemetry(slog.New(slog.NewJSONHandler(&logs, nil)))
	srv := NewServer(upstream.URL, canaryKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/admin/sign-in-policy", nil)
	req.Header.Set("Authorization", "Bearer "+canaryKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1", core.hits)
	}
	if core.lastAuth != "Bearer "+canaryKey {
		t.Errorf("upstream Authorization = %q, want the opaque caller credential", core.lastAuth)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want relayed 200", rec.Code)
	}

	logged := logs.String()
	for _, leaked := range []string{canaryKey, "RequestURI", "request_uri", "Authorization", "authorization"} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: credential material must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	metricsBody, _ := io.ReadAll(metricsRec.Result().Body)
	if strings.Contains(string(metricsBody), canaryKey) {
		t.Errorf("/metrics contains the canary key: credential material must never reach metrics")
	}
}
