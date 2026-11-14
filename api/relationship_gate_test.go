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

// TestRelationshipAuthorizerSharedKeyAllowDeny pins the family gate:
// the shared-key identity allows, anything else denies. One shared
// authorizer covers both G-b wave routes until D8 resolves per-class
// handling.
func TestRelationshipAuthorizerSharedKeyAllowDeny(t *testing.T) {
	t.Parallel()

	if !authorizeRelationshipSharedKey(true) {
		t.Errorf("authorizeRelationshipSharedKey(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeRelationshipSharedKey(false) {
		t.Errorf("authorizeRelationshipSharedKey(false) = allow, want deny (family 403)")
	}
}

// TestRelationshipAuthorizerCountersPinned pins the authorizer
// telemetry for the wave: a terminated shared-key call files one
// allow judgment on the code_relationships family, and the
// unreachable deny branch stays observable.
func TestRelationshipAuthorizerCountersPinned(t *testing.T) {
	t.Parallel()

	for _, leg := range []struct {
		path string
		body string
	}{
		{relationshipPath, relationshipBodies[0]},
		{relationshipStoryPath, relationshipStoryBodies[0]},
	} {
		t.Run("POST "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"entity_id":"e1"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, relationshipAuthedPost(leg.path, leg.body, "application/json"))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.authorizer[relationshipFamily][decisionAllow]; got != 1 {
				t.Errorf("authorizer[%s][allow] = %d, want 1", relationshipFamily, got)
			}
			if got := snap.authorizer[relationshipFamily][decisionDeny]; got != 0 {
				t.Errorf("authorizer[%s][deny] = %d, want 0 on the allow path", relationshipFamily, got)
			}
		})
	}
}

// TestRelationshipDenyBranchShapePinned pins the deny shape for the
// day dest gains a scoped/session resolver: a non-AllScopes identity
// is denied with the documented family 403 in both negotiated shapes
// and the core is never reached — for both wave routes.
func TestRelationshipDenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	legs := []struct {
		name string
		path string
		body string
	}{
		{"POST relationships", relationshipPath, relationshipBodies[0]},
		{"POST story", relationshipStoryPath, relationshipStoryBodies[0]},
	}
	for _, leg := range legs {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.name + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				req := relationshipAuthedPost(leg.path, leg.body, accept)
				rec := httptest.NewRecorder()
				srv.serveRelationshipTerminatedAs(rec, req, false)

				res := rec.Result()
				if res.StatusCode != http.StatusForbidden {
					t.Fatalf("status = %d, want the documented 403", res.StatusCode)
				}
				body, _ := io.ReadAll(res.Body)
				if !strings.Contains(string(body), relationshipFamilyDenyMessage) {
					t.Errorf("body = %q, want it to carry the documented deny text", body)
				}
			})
		}
	}
}

// TestRelationshipWaveRelayExplicitForUnresolvableIdentities pins the
// gated forward-model never-invent rule for the wave: each G-b route
// carrying a credential class dest cannot resolve (a scoped token, an
// OIDC bearer, a browser session cookie) must relay explicitly to the
// core — the core judges, core bytes come back untouched — and must
// never answer a dest-invented 401, 403, 404, 409, 422, or 500.
func TestRelationshipWaveRelayExplicitForUnresolvableIdentities(t *testing.T) {
	t.Parallel()

	legs := []struct {
		name string
		path string
		body string
	}{
		{"POST relationships", relationshipPath, relationshipBodies[0]},
		{"POST story", relationshipStoryPath, relationshipStoryBodies[0]},
	}
	for _, leg := range legs {
		for _, tc := range []struct {
			name   string
			header map[string]string
		}{
			{"scoped bearer", map[string]string{"Authorization": "Bearer scoped-token-123"}},
			{"oidc bearer", map[string]string{"Authorization": "Bearer eyJhbGciOiJSUzI1NiJ9.oidc"}},
			{"browser session", map[string]string{"Cookie": "eshu_session=sess_123"}},
			{"host-prefixed session", map[string]string{"Cookie": "__Host-eshu_session=secure"}},
		} {
			t.Run(leg.name+" "+tc.name, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"entity_id":"e1"}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				req := httptest.NewRequest(http.MethodPost, leg.path, strings.NewReader(leg.body))
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
}

// TestRelationshipWaveGateEmptyKey401 pins the fail-closed gate: with
// no configured key every wave call answers the standing dest 401
// with zero core contact, for the bare and slash spellings of both
// routes.
func TestRelationshipWaveGateEmptyKey401(t *testing.T) {
	t.Parallel()

	targets := []struct {
		path string
		body string
	}{
		{relationshipPath, relationshipBodies[0]},
		{relationshipStoryPath, relationshipStoryBodies[0]},
	}
	for _, target := range targets {
		for _, slash := range []string{"", "/"} {
			t.Run("POST "+target.path+slash, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusOK, body: `{"entity_id":"e1"}`}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, "")
				handler := srv.Handler(DefaultV0Sunset)

				req := relationshipAuthedPost(target.path, target.body, "application/json")
				req.URL.Path = target.path + slash
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
}

// TestRelationshipWaveJunkCookieStays401 pins the D2 follow-up lesson
// for the wave: a Cookie header with no session-cookie member is not
// a credential and answers the standing dest 401 with zero core
// contact.
func TestRelationshipWaveJunkCookieStays401(t *testing.T) {
	t.Parallel()

	for _, leg := range []struct {
		path string
		body string
	}{
		{relationshipPath, relationshipBodies[0]},
		{relationshipStoryPath, relationshipStoryBodies[0]},
	} {
		t.Run("POST "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"entity_id":"e1"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, leg.path, strings.NewReader(leg.body))
			req.Header.Set("Cookie", "theme=dark")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401 for a junk cookie", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (junk cookie never reaches the core)", core.hits)
			}
		})
	}
}

// TestRelationshipWaveCredentiallessStays401 pins that a caller with
// no credential at all answers the standing dest 401 with zero core
// contact on both wave routes.
func TestRelationshipWaveCredentiallessStays401(t *testing.T) {
	t.Parallel()

	for _, leg := range []struct {
		path string
		body string
	}{
		{relationshipPath, relationshipBodies[0]},
		{relationshipStoryPath, relationshipStoryBodies[0]},
	} {
		t.Run("POST "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"entity_id":"e1"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodPost, leg.path, strings.NewReader(leg.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (credential-less never reaches the core)", core.hits)
			}
		})
	}
}

// TestRelationshipTerminated_DeadCore503SingleAttempt pins the
// resilience floor for the wave: with no core origin configured both
// G-b routes answer the permanent local 503 (never Retry-After) with
// zero upstream contact and exactly one dispatch outcome each.
func TestRelationshipTerminated_DeadCore503SingleAttempt(t *testing.T) {
	t.Parallel()

	for _, leg := range []struct {
		path string
		body string
	}{
		{relationshipPath, relationshipBodies[0]},
		{relationshipStoryPath, relationshipStoryBodies[0]},
	} {
		t.Run("POST "+leg.path, func(t *testing.T) {
			t.Parallel()

			tel := NewTelemetry(nil)
			srv := NewServer("", skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, relationshipAuthedPost(leg.path, leg.body, "application/json"))

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want the permanent local 503", rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Retry-After = %q, want empty (local 503 never carries it)", got)
			}
			snap := tel.snapshot()
			if got := snap.dispatches[relationshipFamily][viaNative][outcomeLocal503]; got != 1 {
				t.Errorf("family %q native local_503 = %d, want 1", relationshipFamily, got)
			}
		})
	}
}

// TestRelationshipTerm_LaneFileHygiene is the no-core-import grep
// proof at the source level: the lane files carry no dest-side store,
// port, or table symbol (the pin's graph and content readers plus the
// out-of-pin file-content, ledger, summary, and snapshot legs all stay
// core-side), no query/form/path parser, no request-URI sighting, no
// body-key sighting, no logger/tracer reference, and no serialization
// primitive or lock. A store handle, parse, log line, or lock added to
// api/relationship_terminate.go or api/relationship_wave.go fails here
// until it is removed — the stores-stay-core and opaque-bytes gates
// are enforced by this test, not by review alone.
// scripts/check-parity.sh gates the same list.
func TestRelationshipTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"relationship_terminate.go", "relationship_wave.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, forbidden := range []string{
			"postgres",
			"CodeRelationshipReadAdmission",
			"GraphQuery",
			"ContentStore",
			"ContentReader",
			"ReadStore",
			"GetEntityContent",
			"SearchEntitiesByName",
			"ListRepoFiles",
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
			`"entity_id"`,
			`"transitive"`,
			`"query_type"`,
			`"relationship_type"`,
			`"direction"`,
			`"max_depth"`,
			`"limit"`,
			`"offset"`,
			"slog.",
			"tracing.",
			"span.",
			"json.Marshal",
			"sync.",
			"Mutex",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains %q: lane must stay store-free, parse-free, log-free, and lock-free", file, forbidden)
			}
		}
	}
}
