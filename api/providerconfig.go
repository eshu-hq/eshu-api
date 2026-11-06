// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// AdminProviderConfigMutations native-dispatch routes (tier C-b): the six
// admin provider-config mutation routes from the in-tree
// MutationHandler.Mount at the provenance pin (9938169) and
// go/cmd/api/CUTOVER_OWNERSHIP.md — create, update, revert, enable,
// disable, and test-connection under /api/v0/auth/admin/provider-configs.
// They are registered explicitly inside RequireAuth, ahead of the generic
// /api/ proxy, with the AllScopes family authorizer; the family's reads
// (list, detail, revisions GETs) and every other family still cross the
// byte-relay. Route registration has no path-key collision: Go matches
// method and path together, so the create and update POSTs ride the
// native route while the list and detail GETs on the same path keys
// stay on the generic relay. The OpenAPI fragment is the only surface
// where the core path-key convention bites — the create/update POST
// entries share path keys with the family GETs, so per the core OpenAPI
// convention they live in the reads fragment, which this lane does not
// port (reads stay relay); see the deferred-split disclosure in
// contracts/provider-config-admission.md.
const (
	providerConfigCreateRoute         = "POST /api/v0/auth/admin/provider-configs"
	providerConfigUpdateRoute         = "POST /api/v0/auth/admin/provider-configs/{provider_config_id}"
	providerConfigRevertRoute         = "POST /api/v0/auth/admin/provider-configs/{provider_config_id}/revert"
	providerConfigEnableRoute         = "POST /api/v0/auth/admin/provider-configs/{provider_config_id}/enable"
	providerConfigDisableRoute        = "POST /api/v0/auth/admin/provider-configs/{provider_config_id}/disable"
	providerConfigTestConnectionRoute = "POST /api/v0/auth/admin/provider-configs/{provider_config_id}/test-connection"
)

// providerConfigRoutes lists every native-dispatch route of the
// AdminProviderConfigMutations family in registration order.
var providerConfigRoutes = []string{
	providerConfigCreateRoute,
	providerConfigUpdateRoute,
	providerConfigRevertRoute,
	providerConfigEnableRoute,
	providerConfigDisableRoute,
	providerConfigTestConnectionRoute,
}

// providerConfigWriteThrough is the tier C-b native dispatch for the
// admin provider-config mutations: each request crosses an explicit
// per-family route with the AllScopes family authorizer instead of the
// generic byte-relay. The shape is HTTP write-through — method, path,
// query, headers (including the original Authorization header), and
// body forward to the same core route, and core bytes relay back
// unmodified.
//
// Trust model, binding: dest's only credential is the shared key, which
// core maps to AllScopes (so the dest allow branch is the live one and
// core re-judges every forwarded call as backstop — all-scope admin,
// admin tenant scope, and the identity_admin.provider_config_*
// permission catalog — which is also where the live 403s come from).
// Dest has no scoped/session resolver, so the deny branch is unreachable
// in production until one lands; it is recorded as such, not faked,
// and the tests pin its shape for that day. Dest adds no handler
// validation: request/response semantics stay core-owned, and write-only
// secrets stay sealed — no response, log line, or audit event ever
// carries plaintext or sealed ciphertext. Governance audit stays
// core-side on forwarded requests — like sign-in-policy and
// admin-identity, the in-tree provider-config mutation handlers at the
// provenance pin DO emit audit (EventTypeIDPConfigChange for the allowed
// outcome and every denied outcome including the idempotency-conflict
// answers), and they emit from inside the core handlers that still run
// on every forwarded request, so write-through keeps every audit event
// core-side with no gap. Dest emits none. A family whose handlers emit
// audit must never be re-implemented dest-side without a versioned
// audit-append contract; this lane re-implements nothing.
func (s *Server) providerConfigWriteThrough(w http.ResponseWriter, r *http.Request) {
	s.providerConfigWriteThroughAs(w, r, true)
}

// providerConfigWriteThroughAs is the testable core of the tier C-b
// handler: allScopes is the resolved caller scope. The routes always
// pass true today (the shared key maps to AllScopes in core); the
// false branch is the hookup point for the day dest gains a
// scoped/session resolver, and it stays a plain deny until then. Like
// dispatch, it fails closed with the permanent 503 when no core origin
// is configured, so the native routes can never dereference an
// unconfigured origin.
func (s *Server) providerConfigWriteThroughAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	if !authorizeProviderConfigAdmin(allScopes) {
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			"all-scopes operator authorization is required")
		return
	}
	forwardToCore(s, w, r, viaNative)
}

// authorizeProviderConfigAdmin reports whether an identity carrying
// allScopes may call the admin provider-config mutations. The only identity
// dest can construct today is the shared-key holder, which core maps to
// AllScopes, so callers pass true and core re-judges (tenant scope,
// permission catalog) as backstop. The false branch exists for the day
// dest gains a scoped/session resolver; until then it is unreachable in
// production and must stay a plain deny (no invented reason codes, no
// audit widening).
func authorizeProviderConfigAdmin(allScopes bool) bool {
	return allScopes
}
