// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
)

// W3 IaC-6 native routes (Refs eshu-hq/eshu-api#102, part of
// eshu-hq/eshu#9669): the four POST legs cleared on IaCReadAdmission v0
// (eshu-hq/eshu#10455, recorded in contracts/iac-read-admission.md),
// the resources candidate page cleared on
// IaCInventorySearchAdmission v0 (eshu-hq/eshu#10578, recorded beside
// it), and POST /api/v0/iac/dead cleared on the supplemental
// IaCDeadContentAdmission v0 dual pin (eshu-hq/eshu#10642, recorded
// beside them). Each route is registered explicitly inside RequireAuth,
// ahead of the generic /api/ proxy, and forwards to the same core
// route with the caller bytes intact. No IaC read rides the generic
// relay now; the remaining IaC 5 ride W8a on the same pin.
const (
	iacUnmanagedResourcesRoute            = "POST /api/v0/iac/unmanaged-resources"
	iacManagementStatusRoute              = "POST /api/v0/iac/management-status"
	iacManagementStatusExplainRoute       = "POST /api/v0/iac/management-status/explain"
	iacTerraformImportPlanCandidatesRoute = "POST /api/v0/iac/terraform-import-plan/candidates"
	iacDeadRoute                          = "POST /api/v0/iac/dead"
	iacResourcesRoute                     = "GET /api/v0/iac/resources"
)

// iacNativeRoutes is the W3 IaC-6 termination slice: the four POST legs
// cleared on IaCReadAdmission v0 plus the dead POST leg cleared on
// the supplemental IaCDeadContentAdmission v0 dual pin. The router
// registers each with its trailing-slash twin; the resources leg rides
// iacResourcesRoutes below on its own supplemental pin.
var iacNativeRoutes = []string{
	iacUnmanagedResourcesRoute,
	iacManagementStatusRoute,
	iacManagementStatusExplainRoute,
	iacTerraformImportPlanCandidatesRoute,
	iacDeadRoute,
}

// iacResourcesRoutes is the W3 IaC-6 resources termination slice: the
// single candidate-page GET cleared on IaCInventorySearchAdmission v0.
// Kept separate from iacNativeRoutes so the four POST legs keep their
// pin-uniform dispatch-test table and the supplemental pin stays
// visibly its own coverage. The router registers it with its
// trailing-slash twin on the same write-through handler.
var iacResourcesRoutes = []string{
	iacResourcesRoute,
}

// iacWriteThrough is the W3 IaC-6 native dispatch for the six cleared
// read legs (the four IaCReadAdmission v0 POSTs, the dead POST on the
// supplemental IaCDeadContentAdmission v0 dual pin, plus the resources
// candidate-page GET on the supplemental IaCInventorySearchAdmission
// v0 pin): the request crosses an explicit per-family route instead
// of the generic byte-relay. The shape is HTTP write-through — method,
// path, query, headers (including the original Authorization header),
// and body forward to the same core route, and core bytes relay back
// unmodified.
//
// Trust model, binding: dest's only credential is the shared key, which
// core maps to AllScopes, and core re-judges every forwarded call as
// backstop — the scoped-grant checks (scoped-empty short-circuits,
// cross-tenant selector rejection, bindIaCManagementFilterAccess) and
// the fixed failure messages are handler behavior the core handler
// still runs on every forwarded request. Dest carries no family
// authorizer on these read legs on purpose: dest has no scoped/session
// resolver, so a dest-side AllScopes gate would deny scoped callers the
// core would allow — a behavior change (a traffic flip) this lane must
// not make. Dest adds no handler validation: request/response
// semantics stay core-owned. Governance audit stays core-side on
// forwarded requests — these read handlers emit no audit in core and
// dest emits none, so there is no audit gap on these routes.
func (s *Server) iacWriteThrough(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	forwardToCore(s, w, r, viaNative)
}
