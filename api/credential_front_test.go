// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCredentialFrontLitPaths pins the owned allow/relay legs on the
// d3 and sign-in-policy fronts: shared-key allow behaves exactly like
// the dark legacy path (parity, not a new behavior); session allow
// reaches the family authorizer (plain 403 on AllScopes=false, zero
// core contact); foreign classes relay explicitly (one core hit,
// untouched bytes). Together with the matrix these cover every
// outcome arm of both front switches.
func TestCredentialFrontLitPaths(t *testing.T) {
	t.Parallel()

	const d3Route = "/api/v0/auth/admin/roles"
	const signinRoute = "/api/v0/auth/admin/sign-in-policy"

	serve := func(handler http.Handler, method, route string, mutate func(*http.Request)) (int, []byte) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, matrixRequest(method, route, "", mutate))
		res := rec.Result()
		body, _ := io.ReadAll(res.Body)
		return rec.Code, body
	}

	for _, route := range []struct {
		name   string
		method string
		path   string
	}{
		{"d3", http.MethodGet, d3Route},
		{"signin", http.MethodPatch, signinRoute},
	} {
		// Shared-key allow: lit must equal dark byte for byte.
		darkCore := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
		darkMux := http.NewServeMux()
		darkMux.Handle("/", darkCore.handler())
		darkUpstream := httptest.NewServer(darkMux)
		defer darkUpstream.Close()
		darkSrv := NewServer(darkUpstream.URL, skeletonTestKey)
		darkCode, darkBody := serve(darkSrv.Handler(DefaultV0Sunset), route.method, route.path, withBearer(skeletonTestKey))

		store := &matrixSessions{}
		srv, core := matrixServer(t, store)
		handler := srv.Handler(DefaultV0Sunset)
		litCode, litBody := serve(handler, route.method, route.path, withBearer(skeletonTestKey))
		if litCode != darkCode || string(litBody) != string(darkBody) {
			t.Errorf("%s shared-key allow: lit (%d %q) != dark (%d %q), want parity", route.name, litCode, litBody, darkCode, darkBody)
		}
		if store.calls != 0 {
			t.Errorf("%s shared-key allow: store calls = %d, want 0", route.name, store.calls)
		}

		// Session cookie: safe-method routes resolve locally to
		// the authorizer deny (plain 403, zero core contact); the
		// mutating signin route relays for core-side CSRF judgment
		// instead (see signInPolicyWriterGate: no session arm).
		rec := httptest.NewRecorder()
		before := core.hits
		handler.ServeHTTP(rec, matrixRequest(route.method, route.path, "", withCookie("eshu_session", "valid-session")))
		if route.method == http.MethodGet {
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s session allow: status = %d, want 403 (not AllScopes)", route.name, rec.Code)
			}
			if core.hits != before {
				t.Errorf("%s session allow: upstream hits moved, want zero core contact", route.name)
			}
		} else {
			if core.hits != before+1 {
				t.Errorf("%s session cookie: upstream hits moved by %d, want exactly 1 (relay, CSRF stays core-side)", route.name, core.hits-before)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("%s session cookie: status = %d, want core's 200 relayed untouched", route.name, rec.Code)
			}
		}

		// Foreign relay: one core hit, untouched bytes.
		rec = httptest.NewRecorder()
		before = core.hits
		handler.ServeHTTP(rec, matrixRequest(route.method, route.path, "", withBearer("eshu_scoped_token_value")))
		if core.hits != before+1 {
			t.Errorf("%s foreign relay: upstream hits moved by %d, want exactly 1", route.name, core.hits-before)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("%s foreign relay: status = %d, want core's 200 relayed untouched", route.name, rec.Code)
		}
	}

	// Unit legs: nil-request context lookup and unknown-vocabulary
	// fold into the closed deny set.
	if _, ok := AuthContextOf(nil); ok {
		t.Error("AuthContextOf(nil) = true, want false")
	}
	tel := NewTelemetry(nil)
	tel.observeResolver("bogus-outcome")
	if got := tel.snapshot().resolver["deny"]; got != 1 {
		t.Errorf("observeResolver(bogus) deny = %d, want 1 (unknown folds to deny)", got)
	}
}
