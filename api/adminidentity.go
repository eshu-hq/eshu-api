// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// AdminIdentityMutations native-dispatch routes (tier C-b): the five
// tenant-scoped admin identity mutation routes. They are registered
// explicitly inside RequireAuth, ahead of the generic /api/ proxy, with
// the AllScopes family authorizer; the family's reads and every other
// family still cross the byte-relay. Route shapes are confirmed against
// the in-tree MutationHandler.Mount and go/cmd/api/CUTOVER_OWNERSHIP.md
// at the provenance pin (ebaf4dfa): the invitation revoke lives under
// /api/v0/auth/local/invitations, the other four under
// /api/v0/auth/admin.
const (
	adminIdentityRevokeInvitationRoute = "POST /api/v0/auth/local/invitations/{invite_id}/revoke"
	adminIdentityGrantRoleRoute        = "POST /api/v0/auth/admin/role-assignments"
	adminIdentityRevokeRoleRoute       = "POST /api/v0/auth/admin/role-assignments/revoke"
	adminIdentityCreateMappingRoute    = "POST /api/v0/auth/admin/idp-group-mappings"
	adminIdentityDeleteMappingRoute    = "DELETE /api/v0/auth/admin/idp-group-mappings/{mapping_ref}"
)

// adminIdentityRoutes lists every native-dispatch route of the
// AdminIdentityMutations family in registration order.
var adminIdentityRoutes = []string{
	adminIdentityRevokeInvitationRoute,
	adminIdentityGrantRoleRoute,
	adminIdentityRevokeRoleRoute,
	adminIdentityCreateMappingRoute,
	adminIdentityDeleteMappingRoute,
}

// adminIdentityWriteThrough is the tier C-b native dispatch for the
// admin identity mutations: each request crosses an explicit per-family
// route with the AllScopes family authorizer instead of the generic
// byte-relay. The shape is HTTP write-through — method, path, query,
// headers (including the original Authorization header), and body
// forward to the same core route, and core bytes relay back unmodified.
//
// Trust model, binding: dest's only credential is the shared key, which
// core maps to AllScopes (so the dest allow branch is the live one and
// core re-judges every forwarded call as backstop — all-scope admin,
// admin tenant scope, workspace match, and the identity_admin /
// roles_grants permission catalog — which is also where the live 403s
// come from). Dest has no scoped/session resolver, so the deny branch
// is unreachable in production until one lands; it is recorded as such,
// not faked, and the tests pin its shape for that day. Dest adds no
// handler validation: request/response semantics stay core-owned.
// Governance audit stays core-side on forwarded requests — like
// sign-in-policy, the in-tree identity mutation handlers at the
// provenance pin DO emit audit (EventTypeRoleGrantChange for invitation
// and role decisions, EventTypeIDPConfigChange for mapping decisions,
// for the allowed outcome and every denied outcome including the
// idempotency-conflict answers), and they emit from inside the core
// handlers that still run on every forwarded request, so write-through
// keeps every audit event core-side with no gap. Dest emits none. A
// family whose handlers emit audit must never be re-implemented
// dest-side without a versioned audit-append contract; this lane
// re-implements nothing.
func (s *Server) adminIdentityWriteThrough(w http.ResponseWriter, r *http.Request) {
	s.adminIdentityWriteThroughAs(w, r, true)
}

// adminIdentityWriteThroughAs is the testable core of the tier C-b
// handler: allScopes is the resolved caller scope. The routes always
// pass true today (the shared key maps to AllScopes in core); the
// false branch is the hookup point for the day dest gains a
// scoped/session resolver, and it stays a plain deny until then. Like
// dispatch, it fails closed with the permanent 503 when no core origin
// is configured, so the native routes can never dereference an
// unconfigured origin.
func (s *Server) adminIdentityWriteThroughAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	if !authorizeAdminIdentityAdmin(allScopes) {
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			"all-scopes operator authorization is required")
		return
	}
	forwardToCore(s, w, r, viaNative)
}

// authorizeAdminIdentityAdmin reports whether an identity carrying
// allScopes may call the admin identity mutations. The only identity
// dest can construct today is the shared-key holder, which core maps to
// AllScopes, so callers pass true and core re-judges (tenant scope,
// workspace match, permission catalog) as backstop. The false branch
// exists for the day dest gains a scoped/session resolver; until then
// it is unreachable in production and must stay a plain deny (no
// invented reason codes, no audit widening).
func authorizeAdminIdentityAdmin(allScopes bool) bool {
	return allScopes
}
