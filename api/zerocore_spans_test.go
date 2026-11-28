// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"testing"
)

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
