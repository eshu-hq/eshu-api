// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"

	"github.com/eshu-hq/eshu-api/internal/storespans"
)

// Store-query span vocabulary for package api, aliased from
// internal/storespans (the single seam definition). Production
// store-backed leaves emit through RecordStoreSpan on their query
// path; the harness asserts through the same names. Keeping the
// aliases here lets handler-adjacent code read unqualified while the
// seam stays usable from production packages outside api (notably
// internal/adminread, which cannot import api).
const (
	// StoreOpRead marks a store read span (P-READ success needs >= 1).
	StoreOpRead = storespans.StoreOpRead
	// StoreOpWrite marks a store mutation span (P-WRITE pins these).
	StoreOpWrite = storespans.StoreOpWrite
	// StoreSystemPostgreSQL labels PostgreSQL spans.
	StoreSystemPostgreSQL = storespans.StoreSystemPostgreSQL
	// StoreSystemNeo4j labels Neo4j spans.
	StoreSystemNeo4j = storespans.StoreSystemNeo4j
	// StoreSystemContent labels content-store spans.
	StoreSystemContent = storespans.StoreSystemContent
)

type (
	// StoreSpan is one stdlib-only store-query span; see
	// internal/storespans for the contract.
	StoreSpan = storespans.StoreSpan
	// StoreSpanRecorder is the sink store leaves emit spans to; see
	// internal/storespans for the contract.
	StoreSpanRecorder = storespans.StoreSpanRecorder
	// RecordingStoreSpans is the in-memory recorder the harness
	// asserts against; see internal/storespans for the contract.
	RecordingStoreSpans = storespans.RecordingStoreSpans
)

// WithStoreSpanRecorder returns a context carrying rec; see
// internal/storespans for the contract.
func WithStoreSpanRecorder(ctx context.Context, rec StoreSpanRecorder) context.Context {
	return storespans.WithStoreSpanRecorder(ctx, rec)
}

// StoreSpanRecorderFromContext returns the recorder carried by ctx,
// or nil when ctx carries none; see internal/storespans.
func StoreSpanRecorderFromContext(ctx context.Context) StoreSpanRecorder {
	return storespans.StoreSpanRecorderFromContext(ctx)
}

// RecordStoreSpan emits one span to the recorder carried by ctx, or
// drops it when ctx carries none. The nil-safe drop is what lets
// production leaves call this unconditionally from prod paths.
func RecordStoreSpan(ctx context.Context, span StoreSpan) {
	storespans.RecordStoreSpan(ctx, span)
}
