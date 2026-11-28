// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package storespans

import (
	"context"
	"sync"
	"testing"
)

// TestRecordingStoreSpansSnapshotIsACopy pins that Snapshot returns the
// recorded spans by value: mutating the snapshot must not affect later
// snapshots (the harness asserts against snapshots while prod keeps
// recording).
func TestRecordingStoreSpansSnapshotIsACopy(t *testing.T) {
	t.Parallel()

	var rec RecordingStoreSpans
	rec.RecordStoreSpan(StoreSpan{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Name: "q1"})
	first := rec.Snapshot()
	if len(first) != 1 || first[0].Name != "q1" {
		t.Fatalf("Snapshot() = %v, want one span named q1", first)
	}
	first[0].Name = "mutated"
	second := rec.Snapshot()
	if len(second) != 1 || second[0].Name != "q1" {
		t.Fatalf("Snapshot() after mutation = %v, want one span named q1", second)
	}
}

// TestRecordStoreSpanNilSafety pins the prod-path contract: emitting
// with no recorder attached (bare context, nil context, or an explicit
// nil recorder) is a silent no-op, never a panic.
func TestRecordStoreSpanNilSafety(t *testing.T) {
	t.Parallel()

	span := StoreSpan{System: StoreSystemPostgreSQL, Operation: StoreOpRead, Name: "q"}
	RecordStoreSpan(context.Background(), span)
	//nolint:staticcheck // SA1012 would forbid the nil Context, but this
	// line exists to pin the FromContext nil guard that keeps prod
	// emission panic-free; it also holds the coverage floor.
	RecordStoreSpan(nil, span)
	RecordStoreSpan(WithStoreSpanRecorder(context.Background(), nil), span)
}

// TestRecordStoreSpanRoundTrip pins context plumbing: a span emitted
// through a context carrying a recorder lands in that recorder's
// snapshot with system, operation, and name intact.
func TestRecordStoreSpanRoundTrip(t *testing.T) {
	t.Parallel()

	var rec RecordingStoreSpans
	ctx := WithStoreSpanRecorder(context.Background(), &rec)
	RecordStoreSpan(ctx, StoreSpan{System: StoreSystemNeo4j, Operation: StoreOpWrite, Name: "w1"})
	got := rec.Snapshot()
	if len(got) != 1 {
		t.Fatalf("Snapshot() has %d spans, want 1", len(got))
	}
	if got[0] != (StoreSpan{System: StoreSystemNeo4j, Operation: StoreOpWrite, Name: "w1"}) {
		t.Fatalf("Snapshot()[0] = %+v, want neo4j/write/w1", got[0])
	}
}

// TestStoreSpanRecorderFromContextEmpty pins that a bare context
// carries no recorder, so prod emission stays a no-op there.
func TestStoreSpanRecorderFromContextEmpty(t *testing.T) {
	t.Parallel()

	if rec := StoreSpanRecorderFromContext(context.Background()); rec != nil {
		t.Fatalf("StoreSpanRecorderFromContext(bare) = %v, want nil", rec)
	}
}

// TestRecordingStoreSpansConcurrentPins the mutex contract: parallel
// emitters must not race or lose spans (run with -race in CI).
func TestRecordingStoreSpansConcurrent(t *testing.T) {
	t.Parallel()

	var rec RecordingStoreSpans
	ctx := WithStoreSpanRecorder(context.Background(), &rec)
	const writers = 8
	const perWriter = 50
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				RecordStoreSpan(ctx, StoreSpan{System: StoreSystemContent, Operation: StoreOpRead, Name: "c"})
			}
		}()
	}
	wg.Wait()
	if got := len(rec.Snapshot()); got != writers*perWriter {
		t.Fatalf("Snapshot() has %d spans, want %d", got, writers*perWriter)
	}
}
