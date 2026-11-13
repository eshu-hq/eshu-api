// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// registerNativeSingles mounts the singleton AllScopes write-through
// routes: the suppression upsert (pilot), the sign-in-policy PATCH, the
// retirement trigger, and the terminal-failure acknowledgement. The
// blocks moved here verbatim from routes to hold the 500-line file cap;
// behavior is identical and every route still registers ahead of the
// generic proxy with its trailing-slash twin.
func (s *Server) registerNativeSingles(mux *http.ServeMux) {
	// Native-dispatch pilot: the suppression upsert has its explicit
	// per-family route with the AllScopes authorizer. It is registered
	// ahead of the generic proxy; every other family still crosses the
	// byte-relay below.
	mux.Handle(suppressionRoute, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.suppressionWriteThrough))))
	// The trailing-slash spelling is the same upsert (core mounts the
	// exact path and decides; dest keeps both spellings on the native
	// route so the family is never split across dispatch paths).
	mux.Handle(suppressionRoute+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.suppressionWriteThrough))))
	// Native dispatch, SignInPolicyMutations (W7 writers): the admin
	// sign-in policy PATCH terminates natively behind the family
	// credential-class gate (see api/signin.go): shared-key callers
	// ride the native write-through with the AllScopes authorizer,
	// unresolvable credential classes relay explicitly on the relay
	// lane, and credential-less callers keep the standing 401. The
	// trailing slash stays on the native route for the same
	// never-split reason; the family's GETs and every other family
	// still cross the byte-relay below.
	gated := s.instrumented(viaNative, s.signInPolicyWriterGate(http.HandlerFunc(s.signInPolicyWriteThrough)))
	mux.Handle(signInPolicyRoute, gated)
	mux.Handle(signInPolicyRoute+"/", gated)
	// Native dispatch, AdminRetirement (tier C-a): the operator
	// retirement trigger has its explicit per-family route with the
	// AllScopes authorizer, ahead of the generic proxy. The trailing
	// slash stays on the native route for the same never-split reason;
	// every other family still crosses the byte-relay below.
	mux.Handle(retirementRoute, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.retirementWriteThrough))))
	mux.Handle(retirementRoute+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.retirementWriteThrough))))
	// Native dispatch, AdminTerminalFailure (tier C-a): the operator
	// collector-failure acknowledgement POST has its explicit per-family
	// route with the AllScopes authorizer, ahead of the generic proxy.
	// The trailing slash stays on the native route for the same
	// never-split reason; the family's query reads and every other
	// family still cross the byte-relay below.
	mux.Handle(terminalFailureRoute, s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.terminalFailureWriteThrough))))
	mux.Handle(terminalFailureRoute+"/", s.instrumented(viaNative, RequireAuth(s.apiKey, http.HandlerFunc(s.terminalFailureWriteThrough))))
}
