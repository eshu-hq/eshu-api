// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package credresolver owns the N3 unified credential resolver for
// eshu-hq/eshu-api#448: one front that classifies every inbound
// credential — the shared-key bearer, scoped/PostgreSQL identity
// tokens, OIDC bearer [REDACTED], and browser-session cookies — into a
// single resolution outcome. Locally resolvable classes (shared key,
// safe-method browser-session cookies) resolve without core contact;
// foreign classes (non-key bearers, unsafe-method cookies) relay so
// the core judges them; everything else denies with the single
// authentication_required 401 shape, never a 503 and never a
// credential-class-distinguishing answer.
//
// The resolver is dark by default: Enabled=false reproduces today's
// relay-gate behavior byte-for-byte through the legacy gates, which
// stay untouched. Enabled=true activates local resolution for the
// shared key (already local) and for safe-method session cookies
// only; unsafe-method cookie traffic keeps relaying so the core's
// CSRF 403 semantics survive, and no shadow dual-resolve ever runs
// (a resolve performs a last_seen/idle write core-side, so running
// both paths would double-write).
//
// Layering: the SessionResolver port below mirrors the documented
// N2 session-store shape (ResolveBrowserSession) but is defined
// locally so N3 does not depend on the unmerged N2 branch. When N2
// lands, its store satisfies this port without widening it: scoped
// tokens stay relay (no token-resolve port is added here) and OIDC
// stays relay (no JWT/JWKS dependency is added here).
package credresolver

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

// AuthContext is the resolved caller identity the resolver fronts
// for. Tenant and workspace scope travel inside this struct only,
// never in request headers. The zero value carries no scope: only
// the shared-key holder maps to AllScopes today.
type AuthContext struct {
	// SubjectIDHash is the opaque hash of the subject the session
	// belongs to. It never carries the raw subject.
	SubjectIDHash string
	// TenantID scopes the identity to one tenant.
	TenantID string
	// WorkspaceID scopes the identity to one workspace, empty when
	// the session carries no workspace switch.
	WorkspaceID string
	// AllScopes is true for the shared-key holder, which the core
	// maps to AllScopes. Session identities carry false until a
	// scoped resolver proves otherwise.
	AllScopes bool
	// ExpiresAt is the session expiry the store reported.
	ExpiresAt time.Time
}

// SessionResolver is the narrow browser-session port the resolver
// consumes. It mirrors the documented N2 shape: resolve one session
// by hash, optionally enforcing the CSRF binding. The contract is
// fail-closed — every failure (unknown hash, expired, revoked, CSRF
// mismatch, store error) reports ok=false, never an error the caller
// must map: the resolver answers 401 for all of them and never 503.
type SessionResolver interface {
	// ResolveBrowserSession reports the identity bound to sessionHash.
	// csrfHash carries the presented CSRF proof; requireCSRF enforces
	// the binding. now bounds expiry checks.
	ResolveBrowserSession(sessionHash, csrfHash string, requireCSRF bool, now time.Time) (AuthContext, bool)
}

// Outcome names the closed resolution vocabulary: exactly one value
// per request. Allow outcomes ride the native lane, Relay forwards
// explicitly so the core judges the foreign class, and Deny answers
// the single 401 with zero core contact.
type Outcome int

const (
	// OutcomeDeny answers WriteUnauthorized: missing, wrong, expired,
	// revoked, CSRF-mismatched, or store-failed credentials, plus the
	// unconfigured (empty-key) server. Never 503, never class-labeled.
	OutcomeDeny Outcome = iota
	// OutcomeAllowSharedKey admits the shared-key bearer locally.
	OutcomeAllowSharedKey
	// OutcomeAllowSession admits a safe-method browser-session cookie
	// resolved through the SessionResolver port.
	OutcomeAllowSession
	// OutcomeRelay forwards explicitly so the core judges the class
	// dest cannot resolve: non-key bearers (scoped tokens, OIDC) and
	// unsafe-method cookie traffic (CSRF semantics stay core-side).
	OutcomeRelay
)

// String names the outcome for telemetry. The vocabulary is the
// outcome only — allow, relay, deny — with no credential-class label,
// so denial metrics cannot reintroduce disclosure through
// cardinality.
func (o Outcome) String() string {
	switch o {
	case OutcomeAllowSharedKey, OutcomeAllowSession:
		return "allow"
	case OutcomeRelay:
		return "relay"
	default:
		return "deny"
	}
}

// Session cookie names the resolver accepts, matching the core
// caller-credential reader and the D3 hasSessionCookie shape check.
// Names are the wire contract, never secrets, so matching is by name,
// never by value.
var sessionCookieNames = map[string]struct{}{
	"eshu_session":        {},
	"__Host-eshu_session": {},
}

// Resolver classifies inbound credentials. The zero value denies
// everything: use New and set Enabled to activate local resolution.
// It is safe for concurrent use; the clock hook exists for tests.
type Resolver struct {
	apiKey   string
	sessions SessionResolver
	clock    func() time.Time

	// Enabled switches local resolution on. False reproduces the
	// legacy relay-gate behavior: callers must run the legacy path,
	// never consult Resolve. True resolves the shared key and
	// safe-method session cookies locally and relays the rest.
	Enabled bool
}

// New builds a Resolver over apiKey with sessions backing the cookie
// path. A nil sessions port disables cookie resolution: cookie
// traffic falls back to the legacy shape split. An empty apiKey fails
// closed: every credential denies until a key is configured.
func New(apiKey string, sessions SessionResolver) *Resolver {
	return &Resolver{apiKey: apiKey, sessions: sessions, clock: time.Now}
}

// bearerKey extracts the bearer credential. It reports false when the
// Authorization header is absent or is not a Bearer [REDACTED] The key
// itself never leaves this function except by comparison.
func bearerKey(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	key := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	if key == "" {
		return "", false
	}
	return key, true
}

// sessionCookieValue returns the browser-session cookie value when
// present, and whether any Cookie header was carried at all. Junk
// cookies report present=false with hasCookie=true so callers route
// them through the same store lookup as shaped cookies: the lookup
// input differs, the lookup count and the denial shape do not.
func sessionCookieValue(r *http.Request) (value string, hasCookie bool) {
	if r.Header.Get("Cookie") == "" && len(r.Cookies()) == 0 {
		return "", false
	}
	hasCookie = true
	for _, c := range r.Cookies() {
		if _, ok := sessionCookieNames[c.Name]; ok {
			return c.Value, true
		}
	}
	return "", true
}

// safeMethod reports whether the method may resolve a cookie locally.
// Only safe methods skip the CSRF binding (requireCSRF=false);
// unsafe-method cookie traffic relays so the core's CSRF 403
// semantics survive.
func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// now reports the current time through the testable clock.
func (res *Resolver) now() time.Time {
	if res.clock != nil {
		return res.clock()
	}
	return time.Now()
}

// Resolve classifies r into one Outcome and, on allow, the caller
// identity. The order is binding: fail-closed key check first, then
// the shared-key comparison, then the local cookie path (safe methods
// with a configured store only), then relay for foreign classes,
// deny for everything else. Store failures deny — the relay-lane 503
// is unreachable from this path by construction.
func (res *Resolver) Resolve(r *http.Request) (AuthContext, Outcome) {
	if res.apiKey == "" {
		return AuthContext{}, OutcomeDeny
	}
	if got, ok := bearerKey(r); ok {
		// Constant-time comparison: the key is a bearer secret.
		if subtle.ConstantTimeCompare([]byte(got), []byte(res.apiKey)) == 1 {
			return AuthContext{AllScopes: true}, OutcomeAllowSharedKey
		}
		// A non-key Bearer [REDACTED] a foreign credential class
		// (scoped token, OIDC): relay explicitly so the core judges
		// it. A wrong shared key is indistinguishable here and
		// relays too — dest invents no failure for it.
		return AuthContext{}, OutcomeRelay
	}
	value, hasCookie := sessionCookieValue(r)
	if hasCookie && safeMethod(r.Method) && res.sessions != nil {
		// Safe-method cookie traffic resolves locally through one
		// store lookup — shaped and junk alike, so the old
		// shape-based split (junk=local 401 vs shaped=relay) cannot
		// oracle the cookie jar. requireCSRF stays false: safe
		// methods carry no CSRF binding to enforce.
		if ctx, ok := res.sessions.ResolveBrowserSession(value, "", false, res.now()); ok {
			return ctx, OutcomeAllowSession
		}
		return AuthContext{}, OutcomeDeny
	}
	if hasCookie {
		// Unsafe-method cookie traffic, or cookie traffic with no
		// store configured: relay explicitly so the core judges it
		// (and keeps its CSRF 403 semantics).
		return AuthContext{}, OutcomeRelay
	}
	return AuthContext{}, OutcomeDeny
}
