// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// D10 setup-state termination (Refs eshu-hq/eshu#9669, part of
// #6707). GET /api/v0/auth/setup-state terminates dest-side via the
// SetupNeeded port while the three credential POSTs stay tier-C
// HTTP write-through (see setup.go). The seal signal is the single
// SetupNeeded method of the frozen SetupAdmission v0 5-method set
// (contracts/upstream/setup_admission_contract_test.go); dest calls
// no other setup method, parses no credential, and keeps no setup
// state.

// setupNeededPort is the dest-side seam behind the terminated
// setup-state GET: the seal-signal method of the frozen
// SetupAdmission v0 set, with the exact frozen signature so the port
// reads against the vendored contract without a core import.
// Production satisfies it with a core-fetching adapter over
// D0-proven transport (one GET, no retry); tests drive the same
// adapter against the stub core. The three credential POSTs never
// touch this port — their VerifyBootstrapCredential proofs stay
// core-side on every forwarded request.
type setupNeededPort interface {
	// SetupNeeded reports whether the wizard is still reachable: a
	// retrievable (unconsumed) bootstrap credential exists. It
	// mirrors the frozen core method so a re-signed dest port fails
	// review the same way a re-signed core method fails the frozen
	// test.
	SetupNeeded(ctx context.Context) (bool, error)
}

// coreSetupStatePort is the production setupNeededPort: it reads the
// seal signal from the configured core origin with a single GET of
// the canonical core setup-state route. The adapter is
// request-scoped (built per call in serveSetupState, never shared),
// so recording the bootstrap_mode echo on it is not a cache: no
// cross-request state exists. The fetch is anonymous by construction
// — the route is public pre-auth at the core too — and performs no
// retry: one fetch, then the caller fails closed.
type coreSetupStatePort struct {
	server      *Server
	query       string
	correlation string
	traceparent string
	tracestate  string
	accept      string
	mode        string
}

// coreSetupStatePort satisfies setupNeededPort: the terminated GET
// serves from this seam, so a re-signed dest port fails the build
// the same way a re-signed core method fails the frozen test.
var _ setupNeededPort = (*coreSetupStatePort)(nil)

// SetupNeeded implements setupNeededPort over D0-proven transport:
// one anonymous GET of the canonical core route. The bootstrap_mode
// echo rides the same response and is recorded on the request-scoped
// adapter for synthesis; a second call would fetch again (callers
// make exactly one).
func (p *coreSetupStatePort) SetupNeeded(ctx context.Context) (bool, error) {
	state, err := p.server.fetchSetupState(ctx, setupFetchParams{
		query:       p.query,
		correlation: p.correlation,
		traceparent: p.traceparent,
		tracestate:  p.tracestate,
		accept:      p.accept,
	})
	if err != nil {
		return false, err
	}
	p.mode = state.bootstrapMode
	return state.needsSetup, nil
}

// setupFetchParams carries the inbound request values the anonymous
// port fetch is allowed to see: the query string, the guaranteed
// correlation ID, W3C trace context when the caller sent it, and the
// caller Accept for content negotiation. Nothing else crosses — in
// particular no Authorization header and no other caller metadata —
// so no credential can reach the fetch path and telemetry never
// observes one. Core answers the fixed two-key shape regardless of
// Accept today; forwarding preserves negotiation end to end the way
// every other native lane does.
type setupFetchParams struct {
	query       string
	correlation string
	traceparent string
	tracestate  string
	accept      string
}

// setupStateResult is the terminated answer: the seal signal plus
// the bootstrap_mode echo. The mode is deployment config the core
// echoes for console guidance — never a behavior gate dest-side —
// so it rides the same core fetch as opaque data, parsed but never
// judged.
type setupStateResult struct {
	needsSetup    bool
	bootstrapMode string
}

// setupStateBody is the wire shape the terminated GET synthesizes.
// Keys and order match the core SetupStateResponse
// (contracts/upstream/setup-wire-shapes.txt); dest owns the encoding
// from here on, so parity past this lane is semantic (decoded keys),
// not byte-for-byte.
type setupStateBody struct {
	NeedsSetup    bool   `json:"needs_setup"`
	BootstrapMode string `json:"bootstrap_mode"`
}

// fetchSetupState performs the single port fetch: GET of the
// canonical core setup-state route with the caller's query string,
// the guaranteed correlation ID, and W3C trace context when
// supplied. No Authorization header is ever attached — the dest
// shared key is never injected and no caller credential crosses —
// and the twin spelling never crosses either: dest matched the
// route, so the core always sees the single canonical path. A
// non-200 core answer, an unreadable body, or a missing needs_setup
// key is a fetch error: the caller fails closed downstream, never
// with a synthesized 410.
func (s *Server) fetchSetupState(ctx context.Context, params setupFetchParams) (setupStateResult, error) {
	target := strings.TrimRight(s.coreBaseURL, "/") + "/api/v0/auth/setup-state"
	if params.query != "" {
		target += "?" + params.query
	}
	upstream, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return setupStateResult{}, err
	}
	upstream.Header.Set(correlationHeader, params.correlation)
	if params.accept != "" {
		upstream.Header.Set("Accept", params.accept)
	}
	if params.traceparent != "" {
		upstream.Header.Set("traceparent", params.traceparent)
	}
	if params.tracestate != "" {
		upstream.Header.Set("tracestate", params.tracestate)
	}
	upstream.Host = s.core.Host

	resp, err := s.httpClient.Do(upstream)
	if err != nil {
		return setupStateResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return setupStateResult{}, errSetupStateFetch
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return setupStateResult{}, err
	}
	var decoded struct {
		NeedsSetup    *bool  `json:"needs_setup"`
		BootstrapMode string `json:"bootstrap_mode"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return setupStateResult{}, err
	}
	if decoded.NeedsSetup == nil {
		// No seal signal, no answer: defaulting an absent key to
		// false would report a sealed wizard on garbage.
		return setupStateResult{}, errSetupStateFetch
	}
	return setupStateResult{needsSetup: *decoded.NeedsSetup, bootstrapMode: decoded.BootstrapMode}, nil
}

// errSetupStateFetch marks any port-fetch failure the terminated GET
// maps to the permanent local 503. It carries no core detail: a
// store error never reaches the body.
var errSetupStateFetch = errSetupStateFetchType{}

type errSetupStateFetchType struct{}

func (errSetupStateFetchType) Error() string { return "eshu-api: setup-state fetch failed" }

// serveSetupState terminates one setup-state GET for a keyless
// caller: the route is public pre-auth, so there is no gate and no
// authorizer judgment to file. The seal signal comes from the
// SetupNeeded port; dest synthesizes the 200 from it. Every other
// outcome is the permanent local 503 backend_unavailable —
// unconfigured origin, unreachable core, non-200 core answer,
// unreadable body, missing seal key, or caller cancel — with exactly
// one upstream attempt and never a Retry-After. Dest synthesizes no
// 410 on this route: a sealed wizard answers 200 with
// needs_setup:false, exactly like the core handleState it replaces
// on this path (no seal re-check, no audit — the core state handler
// runs neither, so termination loses neither).
func (s *Server) serveSetupState(w http.ResponseWriter, r *http.Request) {
	if s.core == nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, 0, false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core origin is not configured; no core routes are served")
		return
	}
	port := &coreSetupStatePort{
		server:      s,
		query:       r.URL.RawQuery,
		correlation: CorrelationID(r),
		traceparent: r.Header.Get("traceparent"),
		tracestate:  r.Header.Get("tracestate"),
		accept:      r.Header.Get("Accept"),
	}
	fetchStart := time.Now()
	needed, err := port.SetupNeeded(r.Context())
	if err != nil {
		s.dispatchOutcome(r, viaNative, outcomeLocal503, time.Since(fetchStart), false)
		writeError(w, r, http.StatusServiceUnavailable, backendUnavailableCode,
			"eshu-api: core is unavailable")
		return
	}
	s.dispatchOutcome(r, viaNative, outcomeCoreOK, time.Since(fetchStart), false)
	writeJSON(w, http.StatusOK, setupStateBody{
		NeedsSetup:    needed,
		BootstrapMode: port.mode,
	})
}
