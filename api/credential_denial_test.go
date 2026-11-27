// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eshu-hq/eshu-api/internal/credresolver"
)

// matrixSessions is the scripted N3 session port for the denial
// matrix: only "valid-session" resolves; every other hash — unknown,
// revoked, expired — denies, and every call counts so the harness
// pins the lookup discipline (shaped and junk cookies cost exactly
// one lookup each; missing credentials cost none).
type matrixSessions struct {
	calls int
}

func (m *matrixSessions) ResolveBrowserSession(sessionHash, _ string, _ bool, _ time.Time) (credresolver.AuthContext, bool) {
	m.calls++
	if sessionHash == "valid-session" {
		return credresolver.AuthContext{SubjectIDHash: "sub-hash", TenantID: "t1"}, true
	}
	return credresolver.AuthContext{}, false
}

// matrixServer builds the full production handler fronted by an
// enabled N3 resolver over a scripted store, relaying to a stub
// core that answers 200. Callers observe core.hits to prove
// zero-contact denials and explicit relays.
func matrixServer(t *testing.T, store *matrixSessions) (*Server, *stubCore) {
	t.Helper()
	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	t.Cleanup(upstream.Close)
	res := credresolver.New(skeletonTestKey, store)
	res.Enabled = true
	srv := NewServer(upstream.URL, skeletonTestKey).WithCredentialResolver(res)
	return srv, core
}

// matrixRequest builds one matrix request: method and target with a
// pinned correlation ID (so denial bodies are comparable byte for
// byte), a fixed Accept, and one credential mutation applied.
func matrixRequest(method, target, accept string, mutate func(*http.Request)) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("X-Correlation-ID", "matrix-pin-001")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if mutate != nil {
		mutate(req)
	}
	return req
}

// withBearer sets a Bearer credential.
func withBearer(value string) func(*http.Request) {
	return func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+value)
	}
}

// withCookie sets one cookie.
func withCookie(name, value string) func(*http.Request) {
	return func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: name, Value: value})
	}
}

// denialFingerprint renders the comparable denial surface: status,
// WWW-Authenticate, Content-Type, and body. The matrix asserts
// equality on exactly this surface plus the absence of Set-Cookie.
func denialFingerprint(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	res := rec.Result()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read denial body: %v", err)
	}
	if values := res.Header.Values("Set-Cookie"); len(values) != 0 {
		t.Fatalf("denial carries Set-Cookie %q, want none", values)
	}
	return strings.Join([]string{
		res.Status,
		res.Header.Get("WWW-Authenticate"),
		res.Header.Get("Content-Type"),
		string(body),
	}, "\n")
}

// TestCredentialDenialMatrix pins the N3 denial non-disclosure
// contract from eshu-hq/eshu-api#448: missing, wrong, and junk-cookie
// credentials are indistinguishable on the wire. Every locally
// denied row — no credential, malformed bearer, junk cookie, unknown
// session, empty session value, revoked session — answers the same
// 401 status, WWW-Authenticate challenge, Content-Type, and body
// (pinned correlation ID, plain and envelope shapes separately)
// with zero core contact and no Set-Cookie, on every fronted route.
// Foreign classes (non-key bearers, unsafe-method cookies) relay
// explicitly instead: core is contacted exactly once and core bytes
// pass through untouched. No timing padding exists anywhere in the
// mechanism; the structural proof is the fake-store call discipline
// (junk costs the same single lookup as shaped, missing costs none,
// deterministically across runs) plus a loose per-request ceiling.
func TestCredentialDenialMatrix(t *testing.T) {
	t.Parallel()

	const d8Route = "/api/v0/auth/profile"
	const d3Route = "/api/v0/auth/admin/roles"

	denyRows := map[string]func(*http.Request){
		"missing":          nil,
		"malformed bearer": func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") },
		"junk cookie":      withCookie("tracking", "junk"),
		"unknown session":  withCookie("eshu_session", "no-such-session"),
		"empty session":    withCookie("__Host-eshu_session", ""),
		"revoked session":  withCookie("eshu_session", "revoked-session"),
		"expired session":  withCookie("eshu_session", "expired-session"),
	}
	denyRoutes := map[string]string{"d8": d8Route, "d3": d3Route}

	for _, accept := range []string{"", EnvelopeMIME} {
		acceptName := "plain"
		if accept != "" {
			acceptName = "envelope"
		}
		// One baseline per shape across both fronted routes: a d3
		// denial must read exactly like a d8 denial for the same
		// credential class, or the route itself becomes the oracle.
		var baseline string
		for routeName, route := range denyRoutes {
			for rowName, mutate := range denyRows {
				store := &matrixSessions{}
				srv, core := matrixServer(t, store)
				handler := srv.Handler(DefaultV0Sunset)
				before := core.hits
				start := time.Now()
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, matrixRequest(http.MethodGet, route, accept, mutate))
				if elapsed := time.Since(start); elapsed > 2*time.Second {
					t.Errorf("%s %s %s: request took %v, want under the 2s structural ceiling", acceptName, routeName, rowName, elapsed)
				}
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("%s %s %s: status = %d, want 401", acceptName, routeName, rowName, rec.Code)
				}
				if core.hits != before {
					t.Errorf("%s %s %s: upstream hits moved %d->%d, want zero core contact", acceptName, routeName, rowName, before, core.hits)
				}
				fingerprint := denialFingerprint(t, rec)
				if baseline == "" {
					baseline = fingerprint
				} else if fingerprint != baseline {
					t.Errorf("%s %s %s: denial bytes differ from the missing-credential baseline:\n%s\n--- baseline ---\n%s", acceptName, routeName, rowName, fingerprint, baseline)
				}
			}
		}
	}

	t.Run("store call discipline is deterministic", func(t *testing.T) {
		t.Parallel()
		store := &matrixSessions{}
		srv, _ := matrixServer(t, store)
		handler := srv.Handler(DefaultV0Sunset)
		get := func(mutate func(*http.Request)) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, matrixRequest(http.MethodGet, d8Route, "", mutate))
		}
		get(nil)
		get(func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") })
		if store.calls != 0 {
			t.Errorf("store calls after missing+malformed = %d, want 0", store.calls)
		}
		get(withCookie("tracking", "junk"))
		get(withCookie("eshu_session", "unknown"))
		if store.calls != 2 {
			t.Errorf("store calls after junk+shaped = %d, want 2 (junk costs the same lookup as shaped)", store.calls)
		}
		get(withCookie("eshu_session", "unknown"))
		if store.calls != 3 {
			t.Errorf("store calls after repeat = %d, want 3 (no caching, no double-resolve)", store.calls)
		}
	})

	t.Run("foreign classes relay explicitly", func(t *testing.T) {
		t.Parallel()
		relays := map[string]func(*http.Request){
			"scoped token": withBearer("eshu_scoped_token_value"),
			"oidc bearer":  withBearer("eyJhbGciOiJSUzI1NiJ9.cGF5bG9hZA.c2ln"),
			"wrong key":    withBearer("wrong-key"),
		}
		for rowName, mutate := range relays {
			store := &matrixSessions{}
			srv, core := matrixServer(t, store)
			handler := srv.Handler(DefaultV0Sunset)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, matrixRequest(http.MethodGet, d8Route, "", mutate))
			if core.hits != 1 {
				t.Errorf("%s: upstream hits = %d, want 1 (explicit relay so core judges)", rowName, core.hits)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("%s: status = %d, want core's 200 relayed untouched", rowName, rec.Code)
			}
			if store.calls != 0 {
				t.Errorf("%s: store calls = %d, want 0 (foreign bearers never touch the store)", rowName, store.calls)
			}
		}
		// Unsafe-method cookies relay even when valid: CSRF semantics
		// stay core-side, so the core judges shape and value alike.
		// d8Create is a gated POST route, so the front (not route
		// mismatch) owns the decision.
		const d8Create = "/api/v0/auth/browser-session"
		for _, name := range []string{"eshu_session", "tracking"} {
			store := &matrixSessions{}
			srv, core := matrixServer(t, store)
			handler := srv.Handler(DefaultV0Sunset)
			value := "valid-session"
			if name == "tracking" {
				value = "junk"
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, matrixRequest(http.MethodPost, d8Create, "", withCookie(name, value)))
			if core.hits != 1 {
				t.Errorf("unsafe cookie %s: upstream hits = %d, want 1 (relay, CSRF stays core-side)", name, core.hits)
			}
		}
	})

	t.Run("shared key still rides native", func(t *testing.T) {
		t.Parallel()
		store := &matrixSessions{}
		srv, core := matrixServer(t, store)
		handler := srv.Handler(DefaultV0Sunset)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, matrixRequest(http.MethodGet, d8Route, "", withBearer(skeletonTestKey)))
		if core.hits != 1 {
			t.Fatalf("upstream hits = %d, want 1 (native forward)", core.hits)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want core's 200", rec.Code)
		}
		if store.calls != 0 {
			t.Errorf("store calls = %d, want 0 (shared key never touches the store)", store.calls)
		}
	})

	t.Run("valid session reaches the family authorizer", func(t *testing.T) {
		t.Parallel()
		// Session identities carry AllScopes=false, so the
		// previously unreachable authorizer deny branch answers the
		// plain 403 (reason forbidden) with zero core contact — 403
		// stays reserved for authenticated-but-insufficient-scope.
		store := &matrixSessions{}
		srv, core := matrixServer(t, store)
		handler := srv.Handler(DefaultV0Sunset)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, matrixRequest(http.MethodGet, d8Route, "", withCookie("eshu_session", "valid-session")))
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403 (session identity is not AllScopes)", rec.Code)
		}
		if core.hits != 0 {
			t.Errorf("upstream hits = %d, want 0 (deny never dispatches)", core.hits)
		}
		if got := denialFingerprint(t, rec); !strings.Contains(got, `"forbidden"`) {
			t.Errorf("403 body = %q, want the closed forbidden reason", got)
		}
		if _, ok := AuthContextOf(matrixRequest(http.MethodGet, d8Route, "", nil)); ok {
			t.Errorf("AuthContextOf without binding = true, want false")
		}
	})

	t.Run("telemetry keeps the closed outcome vocabulary", func(t *testing.T) {
		t.Parallel()
		store := &matrixSessions{}
		srv, _ := matrixServer(t, store)
		handler := srv.Handler(DefaultV0Sunset)
		serve := func(mutate func(*http.Request)) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, matrixRequest(http.MethodGet, d8Route, "", mutate))
		}
		serve(nil)
		serve(withBearer("wrong-key"))
		serve(withBearer(skeletonTestKey))
		snap := srv.telemetry.snapshot()
		for outcome, want := range map[string]uint64{"deny": 1, "relay": 1, "allow": 1} {
			if got := snap.resolver[outcome]; got != want {
				t.Errorf("resolver[%q] = %d, want %d", outcome, got, want)
			}
		}
		if len(snap.resolver) != 3 {
			t.Errorf("resolver outcomes = %v, want exactly the closed allow/relay/deny set", snap.resolver)
		}
	})
}

// TestCredentialFrontDarkKeepsLegacyBytes pins the dark default: with
// no resolver attached, a shaped-but-unknown session cookie still
// relays (the legacy shape split), while the same request under an
// enabled resolver denies locally with zero core contact.
func TestCredentialFrontDarkKeepsLegacyBytes(t *testing.T) {
	t.Parallel()

	newCore := func() (*stubCore, *httptest.Server) {
		core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		return core, upstream
	}

	core, upstream := newCore()
	defer upstream.Close()
	darkSrv := NewServer(upstream.URL, skeletonTestKey)
	darkHandler := darkSrv.Handler(DefaultV0Sunset)
	rec := httptest.NewRecorder()
	darkHandler.ServeHTTP(rec, matrixRequest(http.MethodGet, "/api/v0/auth/profile", "", withCookie("eshu_session", "unknown")))
	if core.hits != 1 {
		t.Errorf("dark: upstream hits = %d, want 1 (legacy shaped-cookie relay)", core.hits)
	}

	store := &matrixSessions{}
	litSrv, litCore := matrixServer(t, store)
	_ = litCore
	litHandler := litSrv.Handler(DefaultV0Sunset)
	rec = httptest.NewRecorder()
	before := litCore.hits
	litHandler.ServeHTTP(rec, matrixRequest(http.MethodGet, "/api/v0/auth/profile", "", withCookie("eshu_session", "unknown")))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("lit: status = %d, want local 401", rec.Code)
	}
	if litCore.hits != before {
		t.Errorf("lit: upstream hits moved, want zero core contact for unknown sessions")
	}

	// A disabled resolver is dark too: the legacy split runs untouched.
	res := credresolver.New(skeletonTestKey, store)
	darkish := NewServer(upstream.URL, skeletonTestKey).WithCredentialResolver(res)
	darkishHandler := darkish.Handler(DefaultV0Sunset)
	rec = httptest.NewRecorder()
	darkishHandler.ServeHTTP(rec, matrixRequest(http.MethodGet, "/api/v0/auth/profile", "", withCookie("eshu_session", "unknown-2")))
	if core.hits != 2 {
		t.Errorf("disabled: upstream hits = %d, want 2 (disabled front runs the legacy relay)", core.hits)
	}
}

// TestCredentialFrontEmptyKeyFailsClosed pins fail-closed startup:
// with no key configured, keyed, cookie, and missing requests all
// answer the same 401 with zero core contact, front on or off.
func TestCredentialFrontEmptyKeyFailsClosed(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	store := &matrixSessions{}
	res := credresolver.New("", store)
	res.Enabled = true
	srv := NewServer(upstream.URL, "").WithCredentialResolver(res)
	handler := srv.Handler(DefaultV0Sunset)

	var baseline string
	rows := map[string]func(*http.Request){
		"missing":  nil,
		"keyed":    withBearer(skeletonTestKey),
		"cookie":   withCookie("eshu_session", "valid-session"),
		"scoped":   withBearer("eshu_scoped_token_value"),
		"junk":     withCookie("tracking", "junk"),
		"envelope": nil,
	}
	for rowName, mutate := range rows {
		accept := ""
		if rowName == "envelope" {
			accept = EnvelopeMIME
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, matrixRequest(http.MethodGet, "/api/v0/auth/profile", accept, mutate))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401 (fail closed)", rowName, rec.Code)
		}
		fingerprint := denialFingerprint(t, rec)
		if rowName == "envelope" {
			continue
		}
		if baseline == "" {
			baseline = fingerprint
		} else if fingerprint != baseline {
			t.Errorf("%s: denial bytes differ from baseline:\n%s\n--- baseline ---\n%s", rowName, fingerprint, baseline)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (unconfigured never dispatches)", core.hits)
	}
	if store.calls != 0 {
		t.Errorf("store calls = %d, want 0 (unconfigured denies before any lookup)", store.calls)
	}
}
