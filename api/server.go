// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/url"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"

	"github.com/eshu-hq/eshu-api/internal/credresolver"
)

// Server holds the dependencies of the dispatch surface. It answers
// health and readiness itself and proxies the authenticated /api/*
// subtree to the configured core origin (see dispatch.go). The server
// carries no credential: the key gates requests and crosses to the core
// only in the Authorization header, never in logs or health output.
// Telemetry (see telemetry.go) records per-family request, dispatch,
// authorizer, and audit-drop signals plus the structured request log.
type Server struct {
	coreBaseURL string
	apiKey      string
	core        *url.URL
	httpClient  *http.Client
	telemetry   *Telemetry
	credentials *credresolver.Resolver
	// workItems is the reader-role PG source behind the promoted
	// work-items query lane (see api/admin_workitems_terminate.go).
	// Nil is the unconfigured lane: shared-key calls answer the
	// apilib local 503 and never forward.
	workItems apilibadmin.WorkItemStore
	// deadLetters is the reader-role PG source behind the promoted
	// dead-letters query lane (see api/admin_deadletters_terminate.go).
	// Nil is the unconfigured lane: shared-key calls answer the
	// apilib local 503 and never forward.
	deadLetters apilibadmin.DeadLetterStore
	// decisions is the reader-role PG source behind the promoted
	// decisions query lane (see api/admin_decisions_terminate.go).
	// Nil is the unconfigured lane: shared-key calls answer the
	// apilib local 503 and never forward.
	decisions apilibadmin.DecisionStore
	// reindexProgress is the reader-role PG source behind the
	// promoted reindex-progress read lane (see
	// api/admin_reindex_progress_terminate.go). Nil is the
	// unconfigured lane: shared-key calls answer the apilib local
	// 503 and never forward.
	reindexProgress apilibadmin.MaintenanceStore
}

// NewServer builds a Server that dispatches to coreBaseURL and gates
// /api/* behind apiKey. An empty apiKey fails closed: every /api/*
// request is denied until a key is configured. An empty or unparsable
// origin fails closed too: dispatch answers the local 503 until a valid
// origin is configured. Telemetry defaults to a log-silent registry;
// use WithTelemetry to attach a logging one.
func NewServer(coreBaseURL, apiKey string) *Server {
	return &Server{
		coreBaseURL: coreBaseURL,
		apiKey:      apiKey,
		core:        parseCoreOrigin(coreBaseURL),
		httpClient:  dispatchClient(),
		telemetry:   NewTelemetry(nil),
	}
}

// WithTelemetry attaches tel as this server's signal registry and
// returns the server for chaining. The binary wires its structured
// logger here; tests attach capturing or discard registries.
func (s *Server) WithTelemetry(tel *Telemetry) *Server {
	s.telemetry = tel
	return s
}
