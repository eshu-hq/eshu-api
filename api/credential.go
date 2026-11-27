// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"net/http"

	"github.com/eshu-hq/eshu-api/internal/credresolver"
	"github.com/eshu-hq/eshu-api/internal/sessionstore"
)

// authContextKey is the unexported request-context key carrying the
// resolved caller identity. Unexported so no other package can forge
// or read it: tenant and workspace scope travel inside the resolved
// AuthContext only, never in request headers.
type authContextKey struct{}

// AuthContextOf reports the credential-resolver identity bound to r,
// false when the request rode the shared key or the legacy path (the
// shared-key holder maps to AllScopes in the core and needs no dest
// identity struct).
func AuthContextOf(r *http.Request) (sessionstore.AuthContext, bool) {
	if r == nil {
		return sessionstore.AuthContext{}, false
	}
	ctx, ok := r.Context().Value(authContextKey{}).(sessionstore.AuthContext)
	return ctx, ok
}

// withAuthContext binds ctx to r for downstream authorizers.
func withAuthContext(r *http.Request, ctx sessionstore.AuthContext) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), authContextKey{}, ctx))
}

// WithCredentialResolver attaches res as this server's unified
// credential front and returns the server for chaining. A nil or
// disabled resolver reproduces the legacy relay-gate behavior
// byte-for-byte; an enabled one fronts the D3, D8, and sign-in-policy
// writer gates (see credentialFront). The binary wires this from the
// ESHU_LOCAL_AUTH environment; tests attach scripted resolvers.
func (s *Server) WithCredentialResolver(res *credresolver.Resolver) *Server {
	s.credentials = res
	return s
}

// credentialFront consults the unified resolver when one is enabled
// and reports whether it owned the request. It returns owned=false
// when no resolver is enabled so the caller runs the legacy gate
// untouched. When owned, the caller must honor the outcome: allow
// rides the native lane (session identities bind via withAuthContext
// and their AllScopes bit drives the family authorizer, so the
// previously unreachable deny branch stays a plain 403), relay
// forwards explicitly so the core judges the foreign class, and deny
// answers WriteUnauthorized with zero core contact and never a 503.
//
// The front has no wire side effects itself: it files one
// closed-vocabulary telemetry judgment per owned request — allow,
// relay, or deny, with no credential-class label, so denial metrics
// cannot reintroduce disclosure — and returns. Every denial the
// caller answers collapses to the single authentication_required
// 401: missing, wrong, expired, revoked, CSRF-mismatched, and
// store-failed credentials are indistinguishable on the wire.
func (s *Server) credentialFront(r *http.Request) (owned bool, ctx sessionstore.AuthContext, outcome credresolver.Outcome) {
	res := s.credentials
	if res == nil || !res.Enabled {
		return false, sessionstore.AuthContext{}, credresolver.OutcomeDeny
	}
	ctx, outcome = res.Resolve(r)
	if tel := s.telemetry; tel != nil {
		tel.observeResolver(outcome.String())
	}
	return true, ctx, outcome
}
