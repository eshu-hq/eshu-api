// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package adminread is the satellite reader-role PG store behind the
// work-items query native lane (satellite eshu-hq/eshu-api#335), the
// dead-letters query native lane (satellite eshu-hq/eshu-api#324),
// the decisions query native lane (satellite eshu-hq/eshu-api#325),
// the input-invalid-facts query native lane (satellite
// eshu-hq/eshu-api#326), and the reindex-progress read native lane
// (satellite eshu-hq/eshu-api#170). It implements the pinned apilib
// WorkItemStore, DeadLetterStore, DecisionStore, InputInvalidStore,
// and MaintenanceStore ports over bounded SELECTs, issuing no
// writes, no DDL, and no lock or advisory calls — every query is
// grantable to a read-only role, which is the constraint this package
// exists to hold (work-items lane; the dead-letters lane documents
// its own contract in deadletters.go, the decisions lane in
// decisions.go, the input-invalid-facts lane in inputinvalid.go, and
// the reindex-progress lane in reindexprogress.go):
//
//   - the only table touched is fact_work_items, SELECT only;
//   - the only columns read are work_item_id, scope_id,
//     generation_id, stage, domain, status, attempt_count,
//     replay_attempt_baseline, lease_owner, failure_class,
//     failure_message, created_at, updated_at, visible_at, and
//     failure_details — every one a plain column SELECT, grantable
//     to a reader with GRANT SELECT ON fact_work_items;
//   - the operator_note/prior_failure fold re-derives from
//     failure_details in Go (mirroring core
//     applyWorkItemDetails), so no JSONB operator or stored
//     function enters the grant surface.
//
// Reader wiring (out-of-band, never in serving pods):
//
//	CREATE ROLE eshu_api_admin_read WITH LOGIN;
//	GRANT CONNECT ON DATABASE <core> TO eshu_api_admin_read;
//	GRANT USAGE ON SCHEMA public TO eshu_api_admin_read;
//	GRANT SELECT ON fact_work_items TO eshu_api_admin_read;
//	GRANT SELECT ON ingestion_scopes TO eshu_api_admin_read;
//	GRANT SELECT ON projection_decisions TO eshu_api_admin_read;
//	GRANT SELECT ON projection_decision_evidence TO eshu_api_admin_read;
//	GRANT SELECT ON reducer_input_invalid_facts TO eshu_api_admin_read;
//	GRANT SELECT ON runtime_ingester_control TO eshu_api_admin_read;
//	GRANT SELECT ON repository_reindex_requests TO eshu_api_admin_read;
//	GRANT SELECT ON scope_generations TO eshu_api_admin_read;
//
// The ingestion_scopes grant serves only the dead-letters lane
// (satellite eshu-hq/eshu-api#324), which joins it for the collector
// kind: still SELECT-only, still grantable to a reader. The
// projection_decisions and projection_decision_evidence grants serve
// only the decisions lane (satellite eshu-hq/eshu-api#325), which
// reads the pair plus the per-decision fan-out: still SELECT-only,
// still grantable to a reader. The reducer_input_invalid_facts grant
// serves only the input-invalid-facts lane (satellite
// eshu-hq/eshu-api#326), which reads the quarantine join to the
// already-granted ingestion_scopes: still SELECT-only, still
// grantable to a reader. The
// runtime_ingester_control, repository_reindex_requests, and
// scope_generations grants serve only the reindex-progress lane
// (satellite eshu-hq/eshu-api#170): still SELECT-only, still
// grantable to a reader.
//
// The serving pods authenticate AS that role with a per-role
// password granted via the secret store; the DSN travels by
// environment only (ESHU_API_ADMIN_READ_DSN), never by flag. Any future
// column that needs more than SELECT stops here and escalates —
// this package never takes writer credentials.
//
// Pool sizing is the census-recorded replica line at ops-qa scale
// (see docs/decisions/admin-p0-census.md #335): PerReplicaMaxConns
// (default 5) x (MaxReplicas 1 + SurgeReplicas 1) + CoreAPIBudget 0
// + Headroom 10 <= ServerMaxConnections 100. NewStore validates that
// arithmetic and fails closed on any violation, so an oversized
// ESHU_API_ADMIN_READ_MAX_CONNS refuses to start instead of overrunning
// the server ceiling.
package adminread

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apilibadmin "github.com/eshu-hq/eshu/go/apilib/admin"

	"github.com/eshu-hq/eshu-api/internal/sessionstore"
	"github.com/eshu-hq/eshu-api/internal/storespans"
)

// Environment inputs for the reader-role DSN config. The DSN travels
// by environment only (secretKeyRef-mounted, never a flag or a
// default); an empty value fails closed and the lane answers the
// local 503 until the operator mounts it.
const (
	// AdminReadDSNEnv names the reader-role DSN variable.
	AdminReadDSNEnv = "ESHU_API_ADMIN_READ_DSN"
	// AdminReadMaxConnsEnv optionally overrides the per-replica pool
	// ceiling. Empty keeps DefaultMaxConns; any other value must
	// parse to a positive integer inside the census budget.
	AdminReadMaxConnsEnv = "ESHU_API_ADMIN_READ_MAX_CONNS"
)

// DefaultMaxConns is the per-replica reader-pool ceiling the census
// budget sizes: 5 x (1 + 1) + 0 + 10 = 20 <= 100 at ops-qa scale.
const DefaultMaxConns = 5

// Census-pinned replica-line budget inputs (ops-qa scale, carried
// from the sessionstore 002 worked example: replicas=1, surge=1, no
// HPA; the reader line carries no standing core-api budget because it
// rides the read replica, and keeps the same ops headroom of 10).
const (
	budgetServerMaxConns = 100
	budgetCoreAPIBudget  = 0
	budgetHeadroom       = 10
	budgetMaxReplicas    = 1
	budgetSurgeReplicas  = 1
)

// openTimeout bounds pool establishment at startup: a hanging
// database must fail the boot fast, not stall the serving pod.
const openTimeout = 5 * time.Second

// Config is the bounded reader-role DSN config. DSN authenticates as
// the SELECT-only role; MaxConns of 0 keeps DefaultMaxConns.
type Config struct {
	DSN      string
	MaxConns int32
}

// LoadConfigFromEnv reads the reader-role DSN config from the
// environment. An empty DSN fails closed; an explicit MaxConns must
// parse to a positive integer inside the census budget — anything
// else fails closed instead of starting an unbounded or oversized
// pool.
func LoadConfigFromEnv(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	dsn := strings.TrimSpace(getenv(AdminReadDSNEnv))
	if dsn == "" {
		return Config{}, fmt.Errorf("adminread: credential %s unset, refusing to start", AdminReadDSNEnv)
	}
	maxConns := int32(DefaultMaxConns)
	if raw := strings.TrimSpace(getenv(AdminReadMaxConnsEnv)); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("adminread: %s = %q is not a positive integer, refusing to start", AdminReadMaxConnsEnv, raw)
		}
		maxConns = int32(parsed)
	}
	cfg := Config{DSN: dsn, MaxConns: maxConns}
	if err := ReaderBudget(int(maxConns)).Validate(); err != nil {
		return Config{}, fmt.Errorf("adminread: %w", err)
	}
	return cfg, nil
}

// ReaderBudget builds the census-recorded replica-line budget for a
// per-replica ceiling: the same global arithmetic sessionstore
// PoolBudget enforces, with the census-pinned inputs above.
func ReaderBudget(maxConns int) sessionstore.PoolBudget {
	return sessionstore.PoolBudget{
		ServerMaxConnections: budgetServerMaxConns,
		CoreAPIBudget:        budgetCoreAPIBudget,
		Headroom:             budgetHeadroom,
		MaxReplicas:          budgetMaxReplicas,
		SurgeReplicas:        budgetSurgeReplicas,
		PerReplicaMaxConns:   maxConns,
	}
}

// Pool is the query surface Store reads through: *pgxpool.Pool in
// production, fakes in unit tests. It mirrors the core reader seam
// (db.Queryer): the store owns the SQL, the pool owns the wire. It is
// exported so the zero-core proof can drive the real store read path
// over a fake pool without a live database.
type Pool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Close()
}

// pool is the pre-export name kept as an alias so existing call sites
// read unchanged.
type pool = Pool

// Store is the reader-role PG WorkItemStore: one pool authenticated
// as the SELECT-only role, serving the bounded work-items list. It
// satisfies apilibadmin.WorkItemStore.
type Store struct {
	pool pool
}

// newStore binds an open pool. Production opens its pool through
// NewStore; the zero-core proof binds a fake pool through
// NewStoreFromPool to prove the real read path without a live
// database.
func newStore(p pool) *Store {
	return &Store{pool: p}
}

// NewStoreFromPool binds an already-open pool. Production opens its
// pool through NewStore; the proof harness binds a fake pool here so
// the P-READ span evidence can only come from the real store read
// path, never from a fake WorkItemStore.
func NewStoreFromPool(p Pool) *Store {
	return newStore(p)
}

// NewStore opens the reader pool and fails closed on any sizing,
// parse, dial, or ping failure. The pool authenticates as the
// SELECT-only role from cfg; MaxConns of 0 keeps DefaultMaxConns.
func NewStore(ctx context.Context, cfg Config) (*Store, error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("adminread: reader DSN is unset, refusing to start (set %s)", AdminReadDSNEnv)
	}
	maxConns := cfg.MaxConns
	if maxConns == 0 {
		maxConns = DefaultMaxConns
	}
	if err := ReaderBudget(int(maxConns)).Validate(); err != nil {
		return nil, fmt.Errorf("adminread: %w", err)
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("adminread: parse reader DSN: %w", err)
	}
	poolCfg.MaxConns = maxConns
	opened, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("adminread: open reader pool: %w", err)
	}
	openCtx, cancel := context.WithTimeout(ctx, openTimeout)
	defer cancel()
	if err := opened.Ping(openCtx); err != nil {
		opened.Close()
		return nil, fmt.Errorf("adminread: ping reader pool: %w", err)
	}
	return newStore(opened), nil
}

// Close drains the reader pool. The binary holds the store for the
// process lifetime, so this runs at shutdown only.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// ListWorkItems serves the bounded admin inspection read: the same
// SELECT, filters, default limit, ordering, and failure-details fold
// the core reader applies, returned as apilib rows.
func (s *Store) ListWorkItems(ctx context.Context, f apilibadmin.WorkItemFilter) ([]apilibadmin.WorkItem, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("adminread: store is not configured")
	}
	query, args := buildListWorkItemsQuery(f)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("adminread: list work items: %w", err)
	}
	defer rows.Close()
	items, err := scanWorkItems(rows)
	if err != nil {
		return nil, err
	}
	// The P-READ span evidence (satellite eshu-hq/eshu-api#335): one
	// finished postgresql/read span per successful reader query,
	// recorded through the request context. Outside an instrumented
	// chain the emit is a no-op, so the serving path stays safe with
	// no recorder attached.
	storespans.RecordStoreSpan(ctx, storespans.StoreSpan{
		System:    storespans.StoreSystemPostgreSQL,
		Operation: storespans.StoreOpRead,
		Name:      "admin work-items list",
	})
	return items, nil
}

// workItemRows is the minimal row cursor ListWorkItems scans: real
// pgx rows satisfy it, and tests drive scanWorkItems without a live
// database through it.
type workItemRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// buildListWorkItemsQuery renders the bounded reader SELECT: the
// core column set in core order, optional equality/ANY filters, and
// the updated_at DESC + work_item_id ASC page with a LIMIT bind
// marker. An absent limit defaults to 100; selectors trim, like
// the apilib handler already does for its own fields.
func buildListWorkItemsQuery(f apilibadmin.WorkItemFilter) (string, []any) {
	var builder strings.Builder
	builder.WriteString(`
SELECT
    work_item_id,
    scope_id,
    generation_id,
    stage,
    domain,
    status,
    attempt_count,
    replay_attempt_baseline,
    lease_owner,
    failure_class,
    failure_message,
    created_at,
    updated_at,
    visible_at,
    failure_details
FROM fact_work_items
WHERE 1=1
`)
	args := make([]any, 0, 5)
	if len(f.Statuses) > 0 {
		args = append(args, f.Statuses)
		_, _ = fmt.Fprintf(&builder, " AND status = ANY($%d)\n", len(args))
	}
	if value := strings.TrimSpace(f.ScopeID); value != "" {
		args = append(args, value)
		_, _ = fmt.Fprintf(&builder, " AND scope_id = $%d\n", len(args))
	}
	if value := strings.TrimSpace(f.Stage); value != "" {
		args = append(args, value)
		_, _ = fmt.Fprintf(&builder, " AND stage = $%d\n", len(args))
	}
	if value := strings.TrimSpace(f.FailureClass); value != "" {
		args = append(args, value)
		_, _ = fmt.Fprintf(&builder, " AND failure_class = $%d\n", len(args))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)
	_, _ = fmt.Fprintf(&builder, " ORDER BY updated_at DESC, work_item_id ASC LIMIT $%d", len(args))
	return builder.String(), args
}

// scanWorkItems folds queried rows into apilib rows: nullable
// columns stay nil when NULL, and failure_details unfolds into
// OperatorNote/PriorFailure through applyWorkItemDetails.
func scanWorkItems(rows workItemRows) ([]apilibadmin.WorkItem, error) {
	var items []apilibadmin.WorkItem
	for rows.Next() {
		var item apilibadmin.WorkItem
		var leaseOwner sql.NullString
		var failureClass sql.NullString
		var failureMessage sql.NullString
		var visibleAt sql.NullTime
		var failureDetails sql.NullString
		if err := rows.Scan(
			&item.WorkItemID,
			&item.ScopeID,
			&item.GenerationID,
			&item.Stage,
			&item.Domain,
			&item.Status,
			&item.AttemptCount,
			&item.ReplayAttemptBaseline,
			&leaseOwner,
			&failureClass,
			&failureMessage,
			&item.CreatedAt,
			&item.UpdatedAt,
			&visibleAt,
			&failureDetails,
		); err != nil {
			return nil, fmt.Errorf("adminread: scan work item: %w", err)
		}
		applyWorkItemDetails(&item, failureDetails)
		if leaseOwner.Valid {
			item.LeaseOwner = &leaseOwner.String
		}
		if failureClass.Valid {
			item.FailureClass = &failureClass.String
		}
		if failureMessage.Valid {
			item.FailureMessage = &failureMessage.String
		}
		if visibleAt.Valid {
			item.VisibleAt = &visibleAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("adminread: list work items: %w", err)
	}
	return items, nil
}

// workItemDetailsFold is the part of failure_details the admin
// listing reads: the operator note an admin dead-letter or skip
// stored and the prior_failure a supersede, a stale-scope reclaim or
// an operator note kept. Mirrors the core fold field for field.
type workItemDetailsFold struct {
	PriorFailure *struct {
		Status         string `json:"status"`
		FailureClass   string `json:"failure_class"`
		FailureMessage string `json:"failure_message"`
		UpdatedAt      string `json:"updated_at"`
	} `json:"prior_failure"`
}

// workItemNoteFold decodes on its own so a malformed prior_failure
// cannot discard a valid operator_note, and a malformed
// operator_note cannot discard a valid prior_failure.
type workItemNoteFold struct {
	OperatorNote *string `json:"operator_note"`
}

// applyWorkItemDetails fills item.OperatorNote and item.PriorFailure
// from a row's failure_details. failure_details is free text or
// JSON, so anything that is not a JSON object leaves both nil and is
// not an error: a non-JSON row must never fail the listing.
func applyWorkItemDetails(item *apilibadmin.WorkItem, details sql.NullString) {
	text := strings.TrimSpace(details.String)
	if !details.Valid || text == "" || text[0] != '{' {
		return
	}
	var note workItemNoteFold
	if err := json.Unmarshal([]byte(text), &note); err == nil {
		item.OperatorNote = note.OperatorNote
	}
	var parsed workItemDetailsFold
	if err := json.Unmarshal([]byte(text), &parsed); err != nil || parsed.PriorFailure == nil {
		return
	}
	prior := apilibadmin.PriorFailure{
		Status:         strings.TrimSpace(parsed.PriorFailure.Status),
		FailureClass:   strings.TrimSpace(parsed.PriorFailure.FailureClass),
		FailureMessage: strings.TrimSpace(parsed.PriorFailure.FailureMessage),
		UpdatedAt:      strings.TrimSpace(parsed.PriorFailure.UpdatedAt),
	}
	if prior == (apilibadmin.PriorFailure{}) {
		return
	}
	if at, err := time.Parse(time.RFC3339Nano, prior.UpdatedAt); err == nil {
		prior.UpdatedAt = at.UTC().Format(time.RFC3339)
	}
	item.PriorFailure = &prior
}
