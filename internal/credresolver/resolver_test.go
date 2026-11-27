// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package credresolver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eshu-hq/eshu-api/internal/sessionstore"
)

// fakeSessions is a scripted sessionstore.Resolver: hashes in allow
// resolve to a fixed identity, hashes in fail deny with a store
// error, everything else denies, and every call counts so tests pin
// the lookup discipline (shaped and junk cookies cost exactly one
// lookup each; missing credentials cost none).
type fakeSessions struct {
	allow map[string]sessionstore.AuthContext
	fail  map[string]bool
	calls int
}

func (f *fakeSessions) ResolveBrowserSession(_ context.Context, sessionHash, _ string, _ bool, _ time.Time) (sessionstore.AuthContext, bool, error) {
	f.calls++
	if f.fail[sessionHash] {
		return sessionstore.AuthContext{}, false, errors.New("credresolver test: store failure")
	}
	ctx, ok := f.allow[sessionHash]
	return ctx, ok, nil
}

const testKey = "test-shared-key"

// TestSessionSecretHashMatchesCore pins the cookie-to-hash derivation
// byte for byte against the core's BrowserSessionSecretHash
// (go/internal/query/auth/session/cookies.go in the eshu repo):
// trim, blank to "", else "sha256:"+hex(sha256(secret)). Any drift
// silently denies every real session, so the vector is literal.
func TestSessionSecretHashMatchesCore(t *testing.T) {
	t.Parallel()

	if got := sessionSecretHash("valid-session"); got != "sha256:3547c20033d01a8aeb923117229f1d15f03080ace8de294f4ed2e2b7a09703ad" {
		t.Errorf("sessionSecretHash(valid-session) = %q, want the core sha256 vector", got)
	}
	if got := sessionSecretHash("  padded  "); got != sessionSecretHash("padded") {
		t.Errorf("sessionSecretHash(padded) = %q, want whitespace trimmed", got)
	}
	for _, blank := range []string{"", "   "} {
		if got := sessionSecretHash(blank); got != "" {
			t.Errorf("sessionSecretHash(%q) = %q, want blank to hash (store denies it without a DB touch)", blank, got)
		}
	}
}

// TestResolver closes the classification matrix: shared key allows
// locally, safe-method session cookies resolve through exactly one
// store lookup (valid allows, unknown/empty/junk deny with the same
// single lookup, store errors deny and never 503), non-key bearers
// and unsafe-method cookies relay, and missing credentials deny with
// zero store contact.
func TestResolver(t *testing.T) {
	t.Parallel()

	sessionCtx := sessionstore.AuthContext{SubjectIDHash: "sub-hash", TenantID: "t1", AllScopes: false}
	newResolver := func() (*Resolver, *fakeSessions) {
		store := &fakeSessions{allow: map[string]sessionstore.AuthContext{sessionSecretHash("valid-session"): sessionCtx}}
		res := New(testKey, store)
		res.Enabled = true
		return res, store
	}

	t.Run("shared key allows without store contact", func(t *testing.T) {
		t.Parallel()
		res, store := newResolver()
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		req.Header.Set("Authorization", "Bearer "+testKey)
		ctx, outcome := res.Resolve(req)
		if outcome != OutcomeAllowSharedKey {
			t.Fatalf("outcome = %v, want OutcomeAllowSharedKey", outcome)
		}
		if !ctx.AllScopes {
			t.Errorf("AllScopes = false, want true for the shared-key holder")
		}
		if store.calls != 0 {
			t.Errorf("store calls = %d, want 0 (shared key never touches the store)", store.calls)
		}
	})

	t.Run("wrong bearer relays", func(t *testing.T) {
		t.Parallel()
		res, store := newResolver()
		for _, key := range []string{"wrong-key", "eshu_scoped_token_value", "[REDACTED]"} {
			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
			req.Header.Set("Authorization", "Bearer "+key)
			if _, outcome := res.Resolve(req); outcome != OutcomeRelay {
				t.Errorf("key %q: outcome = %v, want OutcomeRelay", key, outcome)
			}
		}
		if store.calls != 0 {
			t.Errorf("store calls = %d, want 0 (foreign bearers never touch the store)", store.calls)
		}
	})

	t.Run("valid session cookie allows with one lookup", func(t *testing.T) {
		t.Parallel()
		res, store := newResolver()
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		req.AddCookie(&http.Cookie{Name: "eshu_session", Value: "valid-session"})
		ctx, outcome := res.Resolve(req)
		if outcome != OutcomeAllowSession {
			t.Fatalf("outcome = %v, want OutcomeAllowSession", outcome)
		}
		if ctx.SubjectIDHash != "sub-hash" || ctx.TenantID != "t1" {
			t.Errorf("identity = %+v, want the stored session identity", ctx)
		}
		if ctx.AllScopes {
			t.Errorf("AllScopes = true, want false for a session identity")
		}
		if store.calls != 1 {
			t.Errorf("store calls = %d, want exactly 1", store.calls)
		}
	})

	t.Run("unknown and junk cookies deny with one lookup each", func(t *testing.T) {
		t.Parallel()
		res, store := newResolver()
		junk := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		junk.AddCookie(&http.Cookie{Name: "tracking", Value: "junk"})
		if _, outcome := res.Resolve(junk); outcome != OutcomeDeny {
			t.Errorf("junk cookie: outcome = %v, want OutcomeDeny", outcome)
		}
		shaped := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		shaped.AddCookie(&http.Cookie{Name: "__Host-eshu_session", Value: "unknown-session"})
		if _, outcome := res.Resolve(shaped); outcome != OutcomeDeny {
			t.Errorf("unknown session: outcome = %v, want OutcomeDeny", outcome)
		}
		empty := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		empty.AddCookie(&http.Cookie{Name: "eshu_session", Value: ""})
		if _, outcome := res.Resolve(empty); outcome != OutcomeDeny {
			t.Errorf("empty session value: outcome = %v, want OutcomeDeny", outcome)
		}
		if store.calls != 3 {
			t.Errorf("store calls = %d, want 3 (junk costs the same lookup as shaped)", store.calls)
		}
	})

	t.Run("store error denies with one lookup and never 503s", func(t *testing.T) {
		t.Parallel()
		store := &fakeSessions{fail: map[string]bool{sessionSecretHash("flaky-session"): true}}
		res := New(testKey, store)
		res.Enabled = true
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		req.AddCookie(&http.Cookie{Name: "eshu_session", Value: "flaky-session"})
		if _, outcome := res.Resolve(req); outcome != OutcomeDeny {
			t.Fatalf("outcome = %v, want OutcomeDeny (store errors deny, never 503)", outcome)
		}
		if store.calls != 1 {
			t.Errorf("store calls = %d, want exactly 1 (no retry on error)", store.calls)
		}
	})

	t.Run("unsafe method cookies relay", func(t *testing.T) {
		t.Parallel()
		res, store := newResolver()
		for _, cookie := range []*http.Cookie{
			{Name: "eshu_session", Value: "valid-session"},
			{Name: "tracking", Value: "junk"},
		} {
			req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/browser-session", nil)
			req.AddCookie(cookie)
			if _, outcome := res.Resolve(req); outcome != OutcomeRelay {
				t.Errorf("cookie %v: outcome = %v, want OutcomeRelay (CSRF stays core-side)", cookie.Name, outcome)
			}
		}
		if store.calls != 0 {
			t.Errorf("store calls = %d, want 0 (unsafe cookies never resolve locally)", store.calls)
		}
	})

	t.Run("missing credentials deny without store contact", func(t *testing.T) {
		t.Parallel()
		res, store := newResolver()
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		if _, outcome := res.Resolve(req); outcome != OutcomeDeny {
			t.Errorf("outcome = %v, want OutcomeDeny", outcome)
		}
		emptyBearer := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		emptyBearer.Header.Set("Authorization", "Bearer ")
		if _, outcome := res.Resolve(emptyBearer); outcome != OutcomeDeny {
			t.Errorf("empty bearer: outcome = %v, want OutcomeDeny", outcome)
		}
		if store.calls != 0 {
			t.Errorf("store calls = %d, want 0", store.calls)
		}
	})

	t.Run("empty key fails closed", func(t *testing.T) {
		t.Parallel()
		store := &fakeSessions{allow: map[string]sessionstore.AuthContext{sessionSecretHash("valid-session"): sessionCtx}}
		res := New("", store)
		res.Enabled = true
		keyed := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		keyed.Header.Set("Authorization", "Bearer "+testKey)
		if _, outcome := res.Resolve(keyed); outcome != OutcomeDeny {
			t.Errorf("keyed with empty configured key: outcome = %v, want OutcomeDeny", outcome)
		}
		cookie := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		cookie.AddCookie(&http.Cookie{Name: "eshu_session", Value: "valid-session"})
		if _, outcome := res.Resolve(cookie); outcome != OutcomeDeny {
			t.Errorf("cookie with empty configured key: outcome = %v, want OutcomeDeny", outcome)
		}
		if store.calls != 0 {
			t.Errorf("store calls = %d, want 0 (unconfigured denies before any lookup)", store.calls)
		}
	})

	t.Run("nil store falls back to relay for cookies", func(t *testing.T) {
		t.Parallel()
		res := New(testKey, nil)
		res.Enabled = true
		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/profile", nil)
		req.AddCookie(&http.Cookie{Name: "eshu_session", Value: "valid-session"})
		if _, outcome := res.Resolve(req); outcome != OutcomeRelay {
			t.Errorf("outcome = %v, want OutcomeRelay (no store, core judges)", outcome)
		}
	})
}

// TestResolverSatisfiesSessionstorePort pins the rewire at compile
// time from the consumer side too: the fake the resolver tests drive
// is a sessionstore.Resolver, so only a real-port fake can back the
// cookie path — no parallel port can slip back in.
func TestResolverSatisfiesSessionstorePort(t *testing.T) {
	t.Parallel()

	var _ sessionstore.Resolver = (*fakeSessions)(nil)
}

// TestOutcomeStringKeepsClosedVocabulary pins the telemetry labels:
// allow, relay, deny — with no credential-class label that could
// reintroduce disclosure through metric cardinality.
func TestOutcomeStringKeepsClosedVocabulary(t *testing.T) {
	t.Parallel()

	cases := map[Outcome]string{
		OutcomeDeny:           "deny",
		OutcomeAllowSharedKey: "allow",
		OutcomeAllowSession:   "allow",
		OutcomeRelay:          "relay",
		Outcome(99):           "deny",
	}
	for outcome, want := range cases {
		if got := outcome.String(); got != want {
			t.Errorf("Outcome(%d).String() = %q, want %q", int(outcome), got, want)
		}
	}
}
