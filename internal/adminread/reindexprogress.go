// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package adminread

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"

	"github.com/eshu-hq/eshu-api/internal/storespans"
)

// Reindex-progress reader (satellite eshu-hq/eshu-api#170, the third
// D3 store-backed promotion, lifting the last Gap 02 trio GET).
// GetReindexState, RepositoryReindexRequestedAt, and
// FullReconcileState serve the fleet progress read over the same
// SELECT-only contract as the work-items and dead-letters lanes: one
// bounded SELECT per method, issuing no writes, no DDL, and no lock
// or advisory calls. The serving pods authenticate as the
// SELECT-only role (see package doc): the reader grant for this lane
// is SELECT on runtime_ingester_control, repository_reindex_requests,
// scope_generations, and ingestion_scopes.
//
// The access clause mirrors the core store field for field: a shared
// key carries no grants (all-scopes), so every query reads unscoped;
// only a future scoped resolver passing grants would restrict it.
// Dest invents no grants of its own. Any column that needs more than
// SELECT stops here and escalates — this package never takes writer
// credentials.

// GetReindexState serves the fleet watermark read: the stored
// reindex_request_requested_at for the ingester, zero when no row
// was ever recorded (core parity: idle without a row, never an
// error).
func (s *Store) GetReindexState(ctx context.Context, ingester string) (apilibadmin.ReindexState, error) {
	if s == nil || s.pool == nil {
		return apilibadmin.ReindexState{}, fmt.Errorf("adminread: store is not configured")
	}
	query, args := buildGetReindexStateQuery(ingester)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return apilibadmin.ReindexState{}, fmt.Errorf("adminread: get reindex state: %w", err)
	}
	defer rows.Close()
	at, err := scanReindexState(rows)
	if err != nil {
		return apilibadmin.ReindexState{}, err
	}
	storespans.RecordStoreSpan(ctx, storespans.StoreSpan{
		System:    storespans.StoreSystemPostgreSQL,
		Operation: storespans.StoreOpRead,
		Name:      "admin reindex-progress fleet",
	})
	return apilibadmin.ReindexState{RequestedAt: at}, nil
}

// RepositoryReindexRequestedAt serves the per-repository watermark
// point read: the stored requested_at of every named scope in UTC,
// keyed by scope ID. Scopes with no request row are absent from the
// map; the caller compares those scopes against the fleet
// watermark. An empty selection reads nothing and answers empty.
func (s *Store) RepositoryReindexRequestedAt(ctx context.Context, scopeIDs []string) (map[string]time.Time, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("adminread: store is not configured")
	}
	if len(scopeIDs) == 0 {
		return map[string]time.Time{}, nil
	}
	query, args := buildRepositoryReindexRequestedAtQuery(scopeIDs)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("adminread: read repository reindex requested at: %w", err)
	}
	defer rows.Close()
	watermarks, err := scanRepositoryWatermarks(rows)
	if err != nil {
		return nil, err
	}
	storespans.RecordStoreSpan(ctx, storespans.StoreSpan{
		System:    storespans.StoreSystemPostgreSQL,
		Operation: storespans.StoreOpRead,
		Name:      "admin reindex-progress per-repository",
	})
	return watermarks, nil
}

// FullReconcileState serves the per-scope progress view: whether a
// full generation activated, the newest activated full's own ingest
// time (a re-projection Ack never moves it), and the active
// generation. A blank scope answers the zero state without querying,
// mirroring the core sweep read.
func (s *Store) FullReconcileState(ctx context.Context, scopeID string) (apilibadmin.FullReconcileState, error) {
	if strings.TrimSpace(scopeID) == "" {
		return apilibadmin.FullReconcileState{}, nil
	}
	if s == nil || s.pool == nil {
		return apilibadmin.FullReconcileState{}, fmt.Errorf("adminread: store is not configured")
	}
	query, args := buildFullReconcileStateQuery(scopeID)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return apilibadmin.FullReconcileState{}, fmt.Errorf("adminread: read full reconcile state: %w", err)
	}
	defer rows.Close()
	state, err := scanFullReconcileState(rows)
	if err != nil {
		return apilibadmin.FullReconcileState{}, err
	}
	storespans.RecordStoreSpan(ctx, storespans.StoreSpan{
		System:    storespans.StoreSystemPostgreSQL,
		Operation: storespans.StoreOpRead,
		Name:      "admin reindex-progress scope",
	})
	return state, nil
}

// progressRows is the minimal row cursor the progress scans read:
// real pgx rows satisfy it, and tests drive the scans without a live
// database through it.
type progressRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// buildGetReindexStateQuery renders the bounded fleet-watermark
// reader SELECT: the requested_at column for one ingester, the row
// the reindex mutation stamps through the database clock.
func buildGetReindexStateQuery(ingester string) (string, []any) {
	return `SELECT COALESCE(reindex_request_requested_at, '0001-01-01'::timestamptz)
FROM runtime_ingester_control
WHERE ingester = $1`, []any{ingester}
}

// buildRepositoryReindexRequestedAtQuery renders the bounded
// per-repository watermark reader SELECT: the stored watermark of
// exactly the named scopes, a point read on the primary key bounded
// by the request.
func buildRepositoryReindexRequestedAtQuery(scopeIDs []string) (string, []any) {
	return `SELECT scope_id, requested_at
FROM repository_reindex_requests
WHERE scope_id = ANY($1::text[])`, []any{scopeIDs}
}

// buildFullReconcileStateQuery renders the bounded per-scope
// progress reader SELECT: the newest activated full generation's own
// ingest time plus the scope's active generation, as plain column
// SELECTs over the same rows the core sweep decides on. The newest
// activated full orders by ingest time (never the re-projected
// time); only activated fulls count, so a superseded-while-pending
// generation never satisfies a scope.
func buildFullReconcileStateQuery(scopeID string) (string, []any) {
	var builder strings.Builder
	builder.WriteString(`SELECT
    (SELECT generations.ingested_at
     FROM scope_generations AS generations
     WHERE generations.scope_id = $1
       AND generations.is_delta = false
       AND generations.activated_at IS NOT NULL
       AND generations.status IN ('active', 'completed', 'superseded')
     ORDER BY generations.ingested_at DESC, generations.generation_id DESC
     LIMIT 1),
    (SELECT scope.active_generation_id
     FROM ingestion_scopes AS scope
     WHERE scope.scope_id = $1)`)
	return builder.String(), []any{scopeID}
}

// scanReindexState folds the fleet row: the instant, or zero when no
// row was ever recorded.
func scanReindexState(rows progressRows) (time.Time, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return time.Time{}, fmt.Errorf("adminread: get reindex state: %w", err)
		}
		return time.Time{}, nil
	}
	var at time.Time
	if err := rows.Scan(&at); err != nil {
		return time.Time{}, fmt.Errorf("adminread: scan reindex state: %w", err)
	}
	if err := rows.Err(); err != nil {
		return time.Time{}, fmt.Errorf("adminread: get reindex state: %w", err)
	}
	return at.UTC(), nil
}

// scanRepositoryWatermarks folds the per-repository rows into UTC
// watermarks keyed by scope ID.
func scanRepositoryWatermarks(rows progressRows) (map[string]time.Time, error) {
	watermarks := map[string]time.Time{}
	for rows.Next() {
		var scopeID string
		var requestedAt time.Time
		if err := rows.Scan(&scopeID, &requestedAt); err != nil {
			return nil, fmt.Errorf("adminread: scan repository reindex requested at: %w", err)
		}
		watermarks[scopeID] = requestedAt.UTC()
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("adminread: read repository reindex requested at: %w", err)
	}
	return watermarks, nil
}

// scanFullReconcileState folds the per-scope row: a NULL stored full
// stays absent, a NULL active generation stays empty.
func scanFullReconcileState(rows progressRows) (apilibadmin.FullReconcileState, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return apilibadmin.FullReconcileState{}, fmt.Errorf("adminread: read full reconcile state: %w", err)
		}
		return apilibadmin.FullReconcileState{}, nil
	}
	var storedAt sql.NullTime
	var activeID sql.NullString
	if err := rows.Scan(&storedAt, &activeID); err != nil {
		return apilibadmin.FullReconcileState{}, fmt.Errorf("adminread: scan full reconcile state: %w", err)
	}
	if err := rows.Err(); err != nil {
		return apilibadmin.FullReconcileState{}, fmt.Errorf("adminread: read full reconcile state: %w", err)
	}
	state := apilibadmin.FullReconcileState{}
	if storedAt.Valid {
		state.HasProjectedFull = true
		state.LastStoredFullAt = storedAt.Time.UTC()
	}
	if activeID.Valid {
		state.ActiveGenerationID = activeID.String
	}
	return state, nil
}
