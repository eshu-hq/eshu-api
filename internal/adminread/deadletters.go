// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package adminread

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"

	"github.com/eshu-hq/eshu-api/internal/storespans"
)

// Dead-letter reader (satellite eshu-hq/eshu-api#324, the second D3
// store-backed promotion). ListDeadLetterWorkItems serves the bounded
// dead-letter inspection read over the same SELECT-only contract as
// the work-items lane: one bounded SELECT joining fact_work_items to
// ingestion_scopes for the collector kind, issuing no writes, no DDL,
// and no lock or advisory calls. The serving pods authenticate as the
// SELECT-only role (see package doc): the reader grant for this lane
// is SELECT on fact_work_items plus SELECT on ingestion_scopes.
//
// The access clause mirrors the core store field for field: a shared
// key carries no grants (all-scopes), so the clause is inert on the
// live lane and the query reads unscoped; only a future scoped
// resolver passing grants would restrict it. Dest invents no grants
// of its own.

// ListDeadLetterWorkItems serves the bounded dead-letter inspection
// read: the same SELECT, filters, bound, and ordering the core reader
// applies, returned as apilib rows.
func (s *Store) ListDeadLetterWorkItems(ctx context.Context, f apilibadmin.DeadLetterListFilter) ([]apilibadmin.DeadLetterWorkItem, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("adminread: store is not configured")
	}
	query, args := buildListDeadLetterWorkItemsQuery(f)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("adminread: list dead-letter work items: %w", err)
	}
	defer rows.Close()
	items, err := scanDeadLetters(rows)
	if err != nil {
		return nil, err
	}
	// The P-READ span evidence (satellite eshu-hq/eshu-api#324, the
	// #335 lesson carried over): one finished postgresql/read span
	// per successful reader query, recorded through the request
	// context. Outside an instrumented chain the emit is a no-op, so
	// the serving path stays safe with no recorder attached.
	storespans.RecordStoreSpan(ctx, storespans.StoreSpan{
		System:    storespans.StoreSystemPostgreSQL,
		Operation: storespans.StoreOpRead,
		Name:      "admin dead-letters list",
	})
	return items, nil
}

// deadLetterRows is the minimal row cursor ListDeadLetterWorkItems
// scans: real pgx rows satisfy it, and tests drive scanDeadLetters
// without a live database through it.
type deadLetterRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// buildListDeadLetterWorkItemsQuery renders the bounded dead-letter
// reader SELECT: the core column set in core order over the
// fact_work_items join to ingestion_scopes, optional equality/ANY
// filters, and the updated_at DESC + work_item_id ASC page with a
// LIMIT bind marker. An absent limit defaults to 100; selectors
// trim, like the apilib handler already does for its own fields. The
// access clause only binds when grants are present (never for the
// shared-key lane); empty grants read unscoped.
func buildListDeadLetterWorkItemsQuery(f apilibadmin.DeadLetterListFilter) (string, []any) {
	var builder strings.Builder
	builder.WriteString(`
SELECT
    work.work_item_id,
    work.scope_id,
    work.generation_id,
    work.stage,
    work.domain,
    scope.collector_kind,
    work.attempt_count,
    work.failure_class,
    work.created_at,
    work.updated_at,
    work.visible_at
FROM fact_work_items AS work
JOIN ingestion_scopes AS scope ON scope.scope_id = work.scope_id
WHERE work.status = 'dead_letter'
`)
	args := make([]any, 0, 6)
	if value := strings.TrimSpace(f.FailureClass); value != "" {
		args = append(args, value)
		_, _ = fmt.Fprintf(&builder, " AND work.failure_class = $%d\n", len(args))
	}
	if value := strings.TrimSpace(f.Domain); value != "" {
		args = append(args, value)
		_, _ = fmt.Fprintf(&builder, " AND work.domain = $%d\n", len(args))
	}
	if value := strings.TrimSpace(f.ScopeID); value != "" {
		args = append(args, value)
		_, _ = fmt.Fprintf(&builder, " AND work.scope_id = $%d\n", len(args))
	}
	if value := strings.TrimSpace(f.CollectorKind); value != "" {
		args = append(args, value)
		_, _ = fmt.Fprintf(&builder, " AND scope.collector_kind = $%d\n", len(args))
	}
	if len(f.AllowedRepositoryIDs) > 0 || len(f.AllowedScopeIDs) > 0 {
		args = append(args, f.AllowedRepositoryIDs)
		repoArg := len(args)
		args = append(args, f.AllowedScopeIDs)
		scopeArg := len(args)
		_, _ = fmt.Fprintf(&builder,
			" AND ((scope.scope_kind = 'repository' AND scope.source_key = ANY($%d)) OR work.scope_id = ANY($%d))\n",
			repoArg,
			scopeArg,
		)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)
	_, _ = fmt.Fprintf(&builder, " ORDER BY work.updated_at DESC, work.work_item_id ASC LIMIT $%d", len(args))
	return builder.String(), args
}

// scanDeadLetters folds queried rows into apilib rows: nullable
// columns stay nil when NULL.
func scanDeadLetters(rows deadLetterRows) ([]apilibadmin.DeadLetterWorkItem, error) {
	var items []apilibadmin.DeadLetterWorkItem
	for rows.Next() {
		var item apilibadmin.DeadLetterWorkItem
		var failureClass sql.NullString
		var visibleAt sql.NullTime
		if err := rows.Scan(
			&item.WorkItemID,
			&item.ScopeID,
			&item.GenerationID,
			&item.Stage,
			&item.Domain,
			&item.CollectorKind,
			&item.AttemptCount,
			&failureClass,
			&item.CreatedAt,
			&item.UpdatedAt,
			&visibleAt,
		); err != nil {
			return nil, fmt.Errorf("adminread: scan dead-letter work item: %w", err)
		}
		if failureClass.Valid {
			item.FailureClass = &failureClass.String
		}
		if visibleAt.Valid {
			item.VisibleAt = &visibleAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("adminread: list dead-letter work items: %w", err)
	}
	return items, nil
}
