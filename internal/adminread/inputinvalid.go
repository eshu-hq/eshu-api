// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package adminread

import (
	"context"
	"fmt"
	"strings"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"

	"github.com/eshu-hq/eshu-api/internal/storespans"
)

// Input-invalid-facts reader (satellite eshu-hq/eshu-api#326, the
// fourth D3 store-backed promotion). ListReducerInputInvalidFacts
// serves the bounded quarantine inspection read over the same
// SELECT-only contract as the work-items, dead-letters, and decisions
// lanes: one bounded SELECT joining reducer_input_invalid_facts to
// ingestion_scopes for the scope authorization, issuing no writes, no
// DDL, and no lock or advisory calls. The serving pods authenticate
// as the SELECT-only role (see package doc): the reader grant for
// this lane is SELECT on reducer_input_invalid_facts plus SELECT on
// ingestion_scopes.
//
// The access clause mirrors the core store field for field: a shared
// key carries no grants (all-scopes), so the clause is inert on the
// live lane and the query reads unscoped; only a future scoped
// resolver passing grants would restrict it. Dest invents no grants
// of its own.
//
// The scan keeps the reducer's classification verbatim: every column
// is NOT NULL core-side, so each maps into a plain value — a missing
// field stays classified (never a zero-valued invention), exactly
// like the core fold.

// ListReducerInputInvalidFacts serves the bounded quarantine
// inspection read: the same SELECT, filters, bound, and ordering the
// core reader applies, returned as apilib rows.
func (s *Store) ListReducerInputInvalidFacts(ctx context.Context, f apilibadmin.InputInvalidFactListFilter) ([]apilibadmin.InputInvalidFact, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("adminread: store is not configured")
	}
	query, args := buildListReducerInputInvalidFactsQuery(f)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("adminread: list reducer input_invalid facts: %w", err)
	}
	defer rows.Close()
	items, err := scanInputInvalidFacts(rows)
	if err != nil {
		return nil, err
	}
	// The P-READ span evidence (satellites eshu-hq/eshu-api#324 and
	// #335 carried over): one finished postgresql/read span per
	// successful reader query, recorded through the request context.
	// Outside an instrumented chain the emit is a no-op, so the
	// serving path stays safe with no recorder attached.
	storespans.RecordStoreSpan(ctx, storespans.StoreSpan{
		System:    storespans.StoreSystemPostgreSQL,
		Operation: storespans.StoreOpRead,
		Name:      "admin input-invalid-facts list",
	})
	return items, nil
}

// inputInvalidFactRows is the minimal row cursor
// ListReducerInputInvalidFacts scans: real pgx rows satisfy it, and
// tests drive scanInputInvalidFacts without a live database through
// it.
type inputInvalidFactRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// buildListReducerInputInvalidFactsQuery renders the bounded
// quarantine reader SELECT: the core column set in core order over
// the reducer_input_invalid_facts join to ingestion_scopes, the
// required scope/generation pair, the optional domain and fact_kind
// filters, the scope-authorizing access clause, and the core-exact
// decided_at DESC + fact_id ASC + missing_field ASC page with a LIMIT
// bind marker. An absent limit defaults to 100, like the apilib
// handler already does for its own fields. Selectors pass through
// verbatim in shape but trimmed in value, mirroring core (the apilib
// handler already trims, so the builder trim is inert on the lane);
// the access clause only binds when grants are present (never for
// the shared-key lane), so the live lane reads unscoped, like core.
func buildListReducerInputInvalidFactsQuery(f apilibadmin.InputInvalidFactListFilter) (string, []any) {
	var builder strings.Builder
	builder.WriteString(`
SELECT
    quarantine.fact_id,
    quarantine.fact_kind,
    quarantine.missing_field,
    quarantine.failure_class,
    quarantine.domain,
    quarantine.scope_id,
    quarantine.generation_id,
    quarantine.decided_at
FROM reducer_input_invalid_facts AS quarantine
JOIN ingestion_scopes AS scope ON scope.scope_id = quarantine.scope_id
WHERE quarantine.scope_id = $1
  AND quarantine.generation_id = $2
`)
	args := []any{f.ScopeID, f.GenerationID}
	if value := strings.TrimSpace(f.Domain); value != "" {
		args = append(args, value)
		_, _ = fmt.Fprintf(&builder, " AND quarantine.domain = $%d\n", len(args))
	}
	if value := strings.TrimSpace(f.FactKind); value != "" {
		args = append(args, value)
		_, _ = fmt.Fprintf(&builder, " AND quarantine.fact_kind = $%d\n", len(args))
	}
	if len(f.AllowedRepositoryIDs) > 0 || len(f.AllowedScopeIDs) > 0 {
		args = append(args, f.AllowedRepositoryIDs)
		repoArg := len(args)
		args = append(args, f.AllowedScopeIDs)
		scopeArg := len(args)
		_, _ = fmt.Fprintf(&builder,
			" AND ((scope.scope_kind = 'repository' AND scope.source_key = ANY($%d)) OR quarantine.scope_id = ANY($%d))\n",
			repoArg,
			scopeArg,
		)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)
	_, _ = fmt.Fprintf(&builder,
		" ORDER BY quarantine.decided_at DESC, quarantine.fact_id ASC, quarantine.missing_field ASC LIMIT $%d",
		len(args))
	return builder.String(), args
}

// scanInputInvalidFacts folds queried rows into apilib rows: every
// column maps verbatim (the table holds no NULLs, mirroring the core
// scan), so the reducer's missing_field/failure_class classification
// survives the read unchanged.
func scanInputInvalidFacts(rows inputInvalidFactRows) ([]apilibadmin.InputInvalidFact, error) {
	var items []apilibadmin.InputInvalidFact
	for rows.Next() {
		var item apilibadmin.InputInvalidFact
		if err := rows.Scan(
			&item.FactID,
			&item.FactKind,
			&item.MissingField,
			&item.FailureClass,
			&item.Domain,
			&item.ScopeID,
			&item.GenerationID,
			&item.DecidedAt,
		); err != nil {
			return nil, fmt.Errorf("adminread: scan reducer input_invalid fact: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("adminread: list reducer input_invalid facts: %w", err)
	}
	return items, nil
}
