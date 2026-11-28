// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package storespans is the stdlib-only store-query span seam: the
// OTel-style (db.system, db.operation) pair plus a span name,
// recorded through the request context so the zero-core harness can
// assert store evidence without importing OTEL before that
// dependency lands. Production store-backed leaves emit through
// RecordStoreSpan on their query path; outside an instrumented chain
// the emit is a nil-safe no-op, so prod paths stay safe with no
// recorder attached.
package storespans

import (
	"context"
	"sync"
)

// Store operation verbs carried on the harness store spans. They
// populate the db.operation span attribute until the OTEL dependency
// lands; real SQL verbs replace them when the first compute leaf ports
// its queries. The verbs stay closed so span assertions read as one
// vocabulary across P-READ, P-WRITE, and P-FLOW proofs.
const (
	// StoreOpRead marks a store read span (P-READ success needs >= 1).
	StoreOpRead = "read"
	// StoreOpWrite marks a store mutation span (P-WRITE pins these).
	StoreOpWrite = "write"
)

// Closed store-system labels for span assertions, mirroring the
// ADR-0002 section 1 store set: PostgreSQL, Neo4j, and the content
// store. They populate the db.system span attribute.
//
// Rename risk: newer OTEL semconv prefers db.system.name over
// db.system (Refs eshu-hq/eshu-api#449). The harness uses the
// issue-literal db.system until the OTEL dependency lands, then the
// port renames the attribute in one place.
const (
	// StoreSystemPostgreSQL labels PostgreSQL spans.
	StoreSystemPostgreSQL = "postgresql"
	// StoreSystemNeo4j labels Neo4j spans.
	StoreSystemNeo4j = "neo4j"
	// StoreSystemContent labels content-store spans.
	StoreSystemContent = "content"
)

// StoreSpan is one stdlib-only store-query span: the OTel-style
// (db.system, db.operation) pair plus a span name. It is a seam, not a
// sampler: production store-backed leaves record these through the
// context recorder so the zero-core harness can assert store evidence
// without importing OTEL before that dependency lands.
type StoreSpan struct {
	// System is the db.system label; one of the StoreSystem* constants.
	System string
	// Operation is the db.operation verb; one of the StoreOp* constants.
	Operation string
	// Name is the span name, usually the query or handler operation.
	Name string
}

// StoreSpanRecorder is the sink store leaves emit store spans to. The
// context-carried form keeps handler signatures unchanged when a
// store-backed leaf lands.
type StoreSpanRecorder interface {
	// RecordStoreSpan records one finished store-query span.
	RecordStoreSpan(StoreSpan)
}

// RecordingStoreSpans is the in-memory StoreSpanRecorder the harness
// asserts against. It mirrors Telemetry (see api/telemetry.go): a
// mutex guarding the registry plus a Snapshot copy for lock-free test
// assertions. It is safe for concurrent use.
type RecordingStoreSpans struct {
	mu    sync.Mutex
	spans []StoreSpan
}

// RecordStoreSpan implements StoreSpanRecorder.
func (r *RecordingStoreSpans) RecordStoreSpan(span StoreSpan) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = append(r.spans, span)
}

// Snapshot returns a copy of the spans recorded so far.
func (r *RecordingStoreSpans) Snapshot() []StoreSpan {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]StoreSpan, len(r.spans))
	copy(out, r.spans)
	return out
}

// storeSpanRecorderKey is the context key carrying the recorder. It is
// unexported so only the harness accessors can read or overwrite it.
type storeSpanRecorderKey struct{}

// WithStoreSpanRecorder returns a context carrying rec. A nil recorder
// is stored as-is; readers treat it as no recording.
func WithStoreSpanRecorder(ctx context.Context, rec StoreSpanRecorder) context.Context {
	return context.WithValue(ctx, storeSpanRecorderKey{}, rec)
}

// StoreSpanRecorderFromContext returns the recorder carried by ctx, or
// nil when ctx carries none. Callers must nil-check: emitting to a nil
// recorder is a no-op by contract, so prod paths stay safe outside the
// instrumented test chain.
func StoreSpanRecorderFromContext(ctx context.Context) StoreSpanRecorder {
	if ctx == nil {
		return nil
	}
	rec, _ := ctx.Value(storeSpanRecorderKey{}).(StoreSpanRecorder)
	return rec
}

// RecordStoreSpan emits one span to the recorder carried by ctx, or
// drops it when ctx carries none. The nil-safe drop is what lets
// production leaves call this unconditionally from prod paths.
func RecordStoreSpan(ctx context.Context, span StoreSpan) {
	if rec := StoreSpanRecorderFromContext(ctx); rec != nil {
		rec.RecordStoreSpan(span)
	}
}
