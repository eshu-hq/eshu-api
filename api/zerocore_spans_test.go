// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"sync"
	"testing"
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
// sampler: future compute leaves record these through the context
// recorder so the zero-core harness can assert store evidence without
// importing OTEL before that dependency lands.
type StoreSpan struct {
	// System is the db.system label; one of the StoreSystem* constants.
	System string
	// Operation is the db.operation verb; one of the StoreOp* constants.
	Operation string
	// Name is the span name, usually the query or handler operation.
	Name string
}

// StoreSpanRecorder is the sink compute leaves emit store spans to. The
// context-carried form keeps handler signatures unchanged when the
// first store-backed leaf lands.
type StoreSpanRecorder interface {
	// RecordStoreSpan records one finished store-query span.
	RecordStoreSpan(StoreSpan)
}

// RecordingStoreSpans is the in-memory StoreSpanRecorder the harness
// asserts against. It mirrors Telemetry (see telemetry.go): a mutex
// guarding the registry plus a Snapshot copy for lock-free test
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
// future leaves call this unconditionally from prod paths.
func RecordStoreSpan(ctx context.Context, span StoreSpan) {
	if rec := StoreSpanRecorderFromContext(ctx); rec != nil {
		rec.RecordStoreSpan(span)
	}
}

// StoreSpanExpectation is one span assertion: count the recorded spans
// matching System+Operation and require Min <= count <= Max.
type StoreSpanExpectation struct {
	// System is the db.system label to match.
	System string
	// Operation is the db.operation verb to match.
	Operation string
	// Min is the minimum matching spans required.
	Min int
	// Max is the maximum matching spans allowed.
	Max int
}

// AssertStoreSpans fails t when the recorder snapshot violates any
// expectation in want. An empty want set asserts silence: no spans at
// all, which is the denial-path contract.
func AssertStoreSpans(t *testing.T, rec *RecordingStoreSpans, want []StoreSpanExpectation) {
	t.Helper()
	assertStoreSpanSlice(t, rec.Snapshot(), want)
}

// assertStoreSpanSlice is AssertStoreSpans over an explicit slice, so
// RunProof can assert on the per-case span delta while sharing one
// failure vocabulary.
func assertStoreSpanSlice(t *testing.T, got []StoreSpan, want []StoreSpanExpectation) {
	t.Helper()
	if len(want) == 0 && len(got) != 0 {
		t.Fatalf("store spans = %d, want 0 (denial path must emit no spans): %v", len(got), got)
	}
	for _, exp := range want {
		count := 0
		for _, span := range got {
			if span.System == exp.System && span.Operation == exp.Operation {
				count++
			}
		}
		if count < exp.Min || count > exp.Max {
			t.Fatalf("store spans (%s, %s) = %d, want in [%d, %d]; all spans: %v",
				exp.System, exp.Operation, count, exp.Min, exp.Max, got)
		}
	}
}

// TestZeroCore_SpanRecorderRoundTrip pins the recorder seam: emit via
// context, read via snapshot, assert via expectation.
func TestZeroCore_SpanRecorderRoundTrip(t *testing.T) {
	t.Parallel()

	rec := &RecordingStoreSpans{}
	ctx := WithStoreSpanRecorder(context.Background(), rec)
	RecordStoreSpan(ctx, StoreSpan{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Name: "fixture read"})

	AssertStoreSpans(t, rec, []StoreSpanExpectation{
		{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Min: 1, Max: 1},
	})
}

// TestZeroCore_SpanEmitWithoutRecorderIsNoOp pins the prod-path safety
// contract: leaves may emit unconditionally, even with no recorder.
func TestZeroCore_SpanEmitWithoutRecorderIsNoOp(t *testing.T) {
	t.Parallel()

	RecordStoreSpan(context.Background(), StoreSpan{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Name: "dropped"})
	// Intentional nil-context probe (Refs eshu-hq/eshu-api#449):
	// RecordStoreSpan must stay total so future leaves can emit
	// unconditionally from prod paths.
	RecordStoreSpan(nil, StoreSpan{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Name: "dropped"}) //nolint:staticcheck

	rec := &RecordingStoreSpans{}
	AssertStoreSpans(t, rec, nil)
}

// TestZeroCore_SpanSnapshotIsACopy pins the Telemetry-mirroring
// snapshot contract: mutating the returned slice cannot corrupt the
// registry a later assertion reads.
func TestZeroCore_SpanSnapshotIsACopy(t *testing.T) {
	t.Parallel()

	rec := &RecordingStoreSpans{}
	rec.RecordStoreSpan(StoreSpan{System: StoreSystemNeo4j, Operation: StoreOpRead, Name: "original"})
	snap := rec.Snapshot()
	snap[0].System = "mutated"

	AssertStoreSpans(t, rec, []StoreSpanExpectation{
		{System: StoreSystemNeo4j, Operation: StoreOpRead, Min: 1, Max: 1},
	})
}
