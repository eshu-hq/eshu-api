// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"
)

// Admin dead-letters query native termination (satellite
// eshu-hq/eshu-api#324, the second D3 store-backed promotion). This
// file owns the serving contract for the one Admin POST the
// promotion lifts out of the D3 wave: the bounded dead-letters
// query. The route rides the untouched D3 wave gate in d3_terminate.go
// (AUTH peer owns that file): a valid shared key terminates locally
// on the pinned apilib body served from the reader-role PG store,
// while foreign credential classes relay explicitly and
// credential-less callers keep the standing 401 — the same matrix
// the wave registration served, with the terminal swapped from the
// core forward to the local store read.
//
// Lane rule (arbiter D-wave ruling, decision 4, via the D3 wave):
// shared-key callers terminate locally with zero core contact;
// credential classes dest cannot resolve (scoped tokens, OIDC
// bearers, browser sessions) relay explicitly — core judges, core
// bytes come back untouched, dest invents no 401, 403, 404, or 500
// for them; credential-less callers get the standing dest 401. A
// missing store answers the apilib local 503 (never a forward); a
// failed store answers the apilib fixed 500 (never the backend
// text). The only other dest-synthesized errors on the terminated
// path are the gate 401, the family 403, the required-field/bounds
// 400s, and the permanent local 503 when no core origin is
// configured (never Retry-After).
//
// Cookie rule (D2 follow-up lesson): only session-cookie-shaped
// credentials relay — the gate reuses hasSessionCookie, so the
// credential matrix is bit-identical to the wave loop this route
// leaves. An empty-valued session cookie stays credential-less
// (dest 401 with zero core contact), exactly as today.
//
// Store rule: the reader is SELECT-only over fact_work_items joined
// to ingestion_scopes for the collector kind (see internal/adminread
// deadletters.go). A nil store is the unconfigured lane, not a
// defect: it answers the apilib 503 locally and never forwards.
//
// Required fields (core parity, enforced dest-side because the
// pinned apilib v0.2.1 defaults instead of requiring): limit (1-500)
// and timeout_ms (1-30000) are required; the bounds and the
// updated_after/updated_before RFC3339/ordering errors carry the
// core-identical detail strings. The timeout also bounds the store
// call through the request context. The updated_after/updated_before
// window is validated for parity and applied: the pinned apilib
// passes the bounds through to the filter, and the reader store
// renders the core-exact updated_at range (>= after, < before).
//
// Admission pin: the apilib dead-letters contract v0 (the pinned
// external apilib v0.2.1, no replace). The served bytes are the
// apilib ServeDeadLettersQuery rendering of the reader rows: {count,
// items} with core field/RFC3339/pointer-omission parity — the
// apilib handler owns that shape, this lane owns only the gate, the
// required-field validation, the timeout bound, the store handle,
// and the telemetry.

// adminDeadLettersRoute is the promoted singleton: the dead-letters
// query POST. Registered ahead of the D3 loops with its
// trailing-slash twin on the same gate (never-split); deeper
// subtrees and non-POST spellings stay on the generic relay.
const adminDeadLettersRoute = "POST /api/v0/admin/dead-letters/query"

// Dead-letter query bounds (core parity, mirrored from the core
// read: deadLetterListMaxLimit 500, deadLetterListMaxTimeout 30s).
const (
	adminDeadLettersMaxLimit     = 500
	adminDeadLettersMaxTimeoutMS = 30000
)

// WithDeadLetterStore attaches store as this server's reader-role
// dead-letter source and returns the server for chaining. A nil store
// is the unconfigured lane: shared-key calls answer the apilib local
// 503 and never forward. The binary wires this from the
// ESHU_API_ADMIN_READ_DSN environment; tests attach fakes.
func (s *Server) WithDeadLetterStore(store apilibadmin.DeadLetterStore) *Server {
	s.deadLetters = store
	return s
}

// registerAdminDeadLettersQuery mounts the promoted route explicitly
// inside the D3 wave gate, ahead of the D3 loops and the generic
// proxy. Lives here (not in routes()) so the shared router stays
// under the file-size cap — the sign-in-policy precedent.
func (s *Server) registerAdminDeadLettersQuery(mux *http.ServeMux) {
	gated := s.instrumented(viaNative, s.d3ReadGate(http.HandlerFunc(s.serveAdminDeadLettersTerminated)))
	mux.Handle(adminDeadLettersRoute, gated)
	mux.Handle(adminDeadLettersRoute+"/", gated)
}

// serveAdminDeadLettersTerminated terminates the dead-letters query
// for a shared-key caller: the family authorizer runs first, the
// required fields validate with core-identical errors, then the
// query serves locally from the reader-role store with no core
// contact.
func (s *Server) serveAdminDeadLettersTerminated(w http.ResponseWriter, r *http.Request) {
	s.serveAdminDeadLettersTerminatedAs(w, r, true)
}

// serveAdminDeadLettersTerminatedAs is the testable core of the
// terminated reader: allScopes is the resolved caller scope. The
// wave gate always passes true today (the shared key maps to
// AllScopes in core); the false branch is the hookup point for the
// day dest gains a scoped/session resolver, and it stays a plain
// deny until then. Like dispatch, it fails closed with the permanent
// 503 when no core origin is configured, so the unconfigured edge
// keeps the wave behavior bit for bit.
func (s *Server) serveAdminDeadLettersTerminatedAs(w http.ResponseWriter, r *http.Request, allScopes bool) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	// The family authorizer judgment counts on both branches, so the
	// unreachable deny branch stays observable from dest signals. A
	// deny never dispatches: it files the denied outcome on the
	// native lane. The authorizer is the shared D3 trust root (see
	// api/d3_terminate.go): one trust root for the admin read family
	// until D8 resolves per-class handling.
	family := classifyFamily(r.URL.Path)
	if !authorizeD3SharedKey(allScopes) {
		if tel := s.telemetry; tel != nil {
			tel.observeAuthorizer(family, decisionDeny)
		}
		s.dispatchOutcome(r, viaNative, outcomeDenied, 0, false)
		writeError(w, r, http.StatusForbidden, http.StatusText(http.StatusForbidden),
			d3FamilyDenyMessage)
		return
	}
	if tel := s.telemetry; tel != nil {
		tel.observeAuthorizer(family, decisionAllow)
	}
	// Local termination: the lane files local_ok (never a core
	// outcome — no core answered) with zero core latency, then serves
	// the apilib bounded-list rendering of the reader rows. A nil
	// store is the unconfigured lane: apilib answers its 503 locally
	// and the call never forwards. The required-field validation runs
	// first with core-identical errors; the timeout also bounds the
	// store call.
	timeoutMS, ok := s.validateAdminDeadLettersBounds(w, r)
	if !ok {
		return
	}
	s.dispatchOutcome(r, viaNative, outcomeLocalOK, 0, false)
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	apilibadmin.ServeDeadLettersQuery(w, r.WithContext(ctx), s.deadLetters, nil)
}

// validateAdminDeadLettersBounds enforces the required
// limit/timeout_ms fields with core-identical detail strings before
// the apilib delegation (the pinned apilib defaults an absent limit
// and ignores timeout_ms; core requires both). A body that is not
// JSON decodes falls through to the apilib 400 untouched, so the
// malformed-body bytes stay apilib-canonical. It restores the
// consumed body for the delegation and reports the validated
// timeout; ok=false means the error is already written.
func (s *Server) validateAdminDeadLettersBounds(w http.ResponseWriter, r *http.Request) (timeoutMS int, ok bool) {
	var raw []byte
	if r.Body != nil {
		var err error
		raw, err = io.ReadAll(r.Body)
		_ = r.Body.Close()
		if err != nil {
			raw = nil
		}
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))

	var req struct {
		Limit         int    `json:"limit"`
		TimeoutMS     int    `json:"timeout_ms"`
		UpdatedAfter  string `json:"updated_after"`
		UpdatedBefore string `json:"updated_before"`
	}
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&req); err != nil {
		// Not JSON the lane can validate: redelegate so the apilib
		// handler renders its canonical bad-body 400.
		s.dispatchOutcome(r, viaNative, outcomeLocalOK, 0, false)
		apilibadmin.ServeDeadLettersQuery(w, r, s.deadLetters, nil)
		return 0, false
	}
	fail := func(message string) (int, bool) {
		// A validation failure still terminates locally (no core
		// outcome), so it files local_ok like every other local
		// answer on this lane.
		s.dispatchOutcome(r, viaNative, outcomeLocalOK, 0, false)
		writeError(w, r, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), message)
		return 0, false
	}
	if req.Limit <= 0 {
		return fail("limit is required and must be between 1 and 500")
	}
	if req.Limit > adminDeadLettersMaxLimit {
		return fail("limit must be <= 500")
	}
	if req.TimeoutMS <= 0 {
		return fail("timeout_ms is required and must be between 1 and 30000")
	}
	if req.TimeoutMS > adminDeadLettersMaxTimeoutMS {
		return fail("timeout_ms must be <= 30000")
	}
	after, err := parseAdminDeadLettersInstant(req.UpdatedAfter, "updated_after")
	if err != nil {
		return fail(err.Error())
	}
	before, err := parseAdminDeadLettersInstant(req.UpdatedBefore, "updated_before")
	if err != nil {
		return fail(err.Error())
	}
	if after != nil && before != nil && !after.Before(*before) {
		return fail("updated_after must be before updated_before")
	}
	return req.TimeoutMS, true
}

// parseAdminDeadLettersInstant parses one optional RFC3339 window
// bound with the core-identical error. Blank stays absent.
func parseAdminDeadLettersInstant(value, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, &adminDeadLettersFieldError{field: field}
	}
	return &at, nil
}

// adminDeadLettersFieldError renders the core-identical "<field>
// must be RFC3339" detail without importing the core contract.
type adminDeadLettersFieldError struct {
	field string
}

func (e *adminDeadLettersFieldError) Error() string {
	return e.field + " must be RFC3339"
}
