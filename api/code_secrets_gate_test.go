// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestCodeSecretsAuthorizerSharedKeyAllowDeny pins the family gate:
// the shared-key identity allows, anything else denies. One shared
// authorizer covers the W6c wave route until D8 resolves per-class
// handling.
func TestCodeSecretsAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeCodeSecretsSharedKey(true) {
		t.Errorf("authorizeCodeSecretsSharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeCodeSecretsSharedKey(false) {
		t.Errorf("authorizeCodeSecretsSharedKey(false) = allow, want deny (family 403)")
	}
}

// TestCodeSecretsAuthorizerCountersPinned pins the authorizer
// telemetry for the wave: a terminated shared-key call files one
// allow judgment on the code_secrets family, and the unreachable
// deny branch stays observable.
func TestCodeSecretsAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	for _, body := range codeSecretsBodies {
		t.Run("POST "+codeSecretsInvestigatePath, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"findings":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeSecretsAuthedPost(codeSecretsInvestigatePath, body, "application/json"))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.authorizer[codeSecretsFamily][decisionAllow]; got != 1 {
				t.Errorf("authorizer[%s][allow] = %d, want 1", codeSecretsFamily, got)
			}
			if got := snap.authorizer[codeSecretsFamily][decisionDeny]; got != 0 {
				t.Errorf("authorizer[%s][deny] = %d, want 0 on the allow path", codeSecretsFamily, got)
			}
		})
	}
}

// TestCodeSecretsDenyBranchShapePinned pins the deny shape for the
// day dest gains a scoped/session resolver: a non-AllScopes identity
// is denied with the documented family 403 in both negotiated shapes
// and the core is never reached.
func TestCodeSecretsDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		t.Run("POST secrets "+accept, func(t *testing.T) {
			t.Parallel()

			req := codeSecretsAuthedPost(codeSecretsInvestigatePath, codeSecretsBodies[0], accept)
			rec := httptest.NewRecorder()
			srv.serveCodeSecretsTerminatedAs(rec, req, false)

			res := rec.Result()
			if res.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want the documented 403", res.StatusCode)
			}
			body, _ := io.ReadAll(res.Body)
			if !strings.Contains(string(body), codeSecretsFamilyDenyMessage) {
				t.Errorf("body = %q, want it to carry the documented deny text", body)
			}
		})
	}
}

// TestCodeSecretsWaveRelayExplicitForUnresolvableIdentities pins the
// gated forward-model never-invent rule for the wave: the W6c route
// carrying a credential class dest cannot resolve (a scoped token,
// an OIDC bearer, a browser session cookie) must relay explicitly to
// the core — the core judges, core bytes come back untouched — and
// must never answer a dest-invented 401, 403, 404, 409, 422, or 500.
func TestCodeSecretsWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
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
		t.Run("POST secrets "+tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"findings":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, codeSecretsInvestigatePath, strings.NewReader(codeSecretsBodies[0]))
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Errorf("%s: upstream hits = %d, want 1 (relay explicitly, never gate-invent)", tc.name, core.hits)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("%s: status = %d, want the relayed core 200 (never 401/403/404/409/422/500-invented)", tc.name, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != core.body {
				t.Errorf("%s: body = %q, want the relayed core bytes %q", tc.name, body, core.body)
			}
			snap := tel.snapshot()
			found := false
			for _, lanes := range snap.dispatches {
				if lanes[viaRelay] != nil {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: dispatches = %v, want a relay lane row (never invented)", tc.name, snap.dispatches)
			}
		})
	}
}

// TestCodeSecretsWaveGateEmptyKey401 pins the fail-closed gate: with
// no configured key every wave call answers the standing dest 401
// with zero core contact, for the bare and slash spellings.
func TestCodeSecretsWaveGateEmptyKey401(t *testing.T) {
	t.Parallel()

	for _, slash := range []string{"", "/"} {
		t.Run("POST "+codeSecretsInvestigatePath+slash, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"findings":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, "")
			handler := srv.Handler(DefaultV0Sunset)

			req := codeSecretsAuthedPost(codeSecretsInvestigatePath, codeSecretsBodies[0], "application/json")
			req.URL.Path = codeSecretsInvestigatePath + slash
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

// TestCodeSecretsWaveJunkCookieStays401 pins the D2 follow-up lesson
// for the wave: a Cookie header with no session-cookie member is not
// a credential and answers the standing dest 401 with zero core
// contact.
func TestCodeSecretsWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"findings":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodPost, codeSecretsInvestigatePath, strings.NewReader(codeSecretsBodies[0]))
	req.Header.Set("Cookie", "theme=dark")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing dest 401 for a junk cookie", rec.Code)
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (junk cookie never reaches the core)", core.hits)
	}
}

// TestCodeSecretsWaveCredentiallessStays401 pins that a caller with
// no credential at all answers the standing dest 401 with zero core
// contact.
func TestCodeSecretsWaveCredentiallessStays401(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"findings":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodPost, codeSecretsInvestigatePath, strings.NewReader(codeSecretsBodies[0]))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want the standing dest 401", rec.Code)
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (credential-less never reaches the core)", core.hits)
	}
}

// TestCodeSecretsTerminated_DeadCore503SingleAttempt pins the
// resilience floor for the wave: with no core origin configured the
// W6c route answers the permanent local 503 (never Retry-After) with
// zero upstream contact and exactly one dispatch outcome.
func TestCodeSecretsTerminated_DeadCore503SingleAttempt(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(nil)
	srv := NewServer("", skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, codeSecretsAuthedPost(codeSecretsInvestigatePath, codeSecretsBodies[0], "application/json"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want the permanent local 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want empty (local 503 never carries it)", got)
	}
	snap := tel.snapshot()
	if got := snap.dispatches[codeSecretsFamily][viaNative][outcomeLocal503]; got != 1 {
		t.Errorf("family %q native local_503 = %d, want 1", codeSecretsFamily, got)
	}
}

// TestCodeSecretsTerm_LaneFileHygiene is the no-core-import grep proof
// at the source level: the lane files carry no dest-side store,
// port, or table symbol (the pin's content readers plus the out-of-pin
// graph, file-content, ledger, summary, and snapshot legs all stay
// core-side), no query/form/path parser, no request-URI sighting, no
// body-key sighting, no logger/tracer reference, and no serialization
// primitive or lock. A store handle, parse, log line, or lock added to
// api/code_secrets_terminate.go or api/code_secrets_wave.go fails here
// until it is removed — the stores-stay-core and opaque-bytes gates
// are enforced by this test, not by review alone.
// scripts/check-parity.sh gates the same list.
func TestCodeSecretsTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"code_secrets_terminate.go", "code_secrets_wave.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, forbidden := range []string{
			"postgres",
			"CodeHardcodedSecretReadAdmission",
			"GraphQuery",
			"ContentStore",
			"ContentReader",
			"ReadStore",
			"InvestigateHardcodedSecrets",
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
			`"repo_id"`,
			`"finding_kinds"`,
			`"limit"`,
			`"offset"`,
			`"include_suppressed"`,
			"slog.",
			"tracing.",
			"span.",
			"json.Marshal",
			"sync.",
			"Mutex",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains %q, want store-free, parse-free, log-free, lock-free", file, forbidden)
			}
		}
	}
}
