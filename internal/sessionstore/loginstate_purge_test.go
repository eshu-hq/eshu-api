// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// purgeScript programs one ExecContext answer: either a row count or
// an error. A scripted error may arrive with a partial count for the
// RowsAffected-error path.
type purgeScript struct {
	rows        int64
	execErr     error
	affectedErr error
}

// purgeFakeDB scripts batched DELETE answers in order and records
// every query with its arguments, so tests pin the cutoff value and
// prove hashes never interpolate into SQL text.
type purgeFakeDB struct {
	scripts []purgeScript
	queries []string
	args    [][]any
	onExec  func(call int)
}

func (f *purgeFakeDB) QueryRowContext(_ context.Context, _ string, _ ...any) Row {
	return &fakeRow{err: errors.New("sessionstore test: purge uses Exec only")}
}

func (f *purgeFakeDB) QueryContext(_ context.Context, _ string, _ ...any) (Rows, error) {
	return nil, errors.New("sessionstore test: purge uses Exec only")
}

func (f *purgeFakeDB) ExecContext(_ context.Context, q string, args ...any) (Result, error) {
	call := len(f.queries)
	f.queries = append(f.queries, q)
	f.args = append(f.args, args)
	if f.onExec != nil {
		f.onExec(call)
	}
	if call >= len(f.scripts) {
		return &fakeResult{n: 0}, nil
	}
	s := f.scripts[call]
	if s.execErr != nil {
		return nil, s.execErr
	}
	return &fakeResult{n: s.rows, err: s.affectedErr}, nil
}

func TestLoginStatePurge_ValidationRejectsBeforeDB(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	cases := map[string]func(now *time.Time, retention *time.Duration, batch *int){
		"zero now":           func(now *time.Time, _ *time.Duration, _ *int) { *now = time.Time{} },
		"zero retention":     func(_ *time.Time, r *time.Duration, _ *int) { *r = 0 },
		"negative retention": func(_ *time.Time, r *time.Duration, _ *int) { *r = -time.Hour },
		"zero batch":         func(_ *time.Time, _ *time.Duration, b *int) { *b = 0 },
		"negative batch":     func(_ *time.Time, _ *time.Duration, b *int) { *b = -1 },
		"over-cap batch":     func(_ *time.Time, _ *time.Duration, b *int) { *b = MaxLoginStatePurgeBatch + 1 },
	}
	for name, mutate := range cases {
		n := now
		r := LoginStateRetention
		b := DefaultLoginStatePurgeBatch
		mutate(&n, &r, &b)
		fake := &purgeFakeDB{scripts: []purgeScript{{rows: 1}}}
		p := NewLoginStatePurger(fake, nil)
		if _, err := p.Purge(context.Background(), n, r, b); err == nil {
			t.Errorf("%s must fail validation", name)
		}
		if len(fake.queries) != 0 {
			t.Errorf("%s must reject before touching the database, got %v", name, fake.queries)
		}
	}
}

func TestLoginStatePurge_DeletesPastCutoffInBatches(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	retention := 7 * 24 * time.Hour
	fake := &purgeFakeDB{scripts: []purgeScript{{rows: 2}, {rows: 2}, {rows: 1}}}
	p := NewLoginStatePurger(fake, nil)
	got, err := p.Purge(context.Background(), now, retention, 2)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if got != 5 {
		t.Fatalf("purge deleted = %d, want 5", got)
	}
	if len(fake.queries) != 3 {
		t.Fatalf("purge batches = %d, want 3", len(fake.queries))
	}
	wantCutoff := now.Add(-retention)
	for i, q := range fake.queries {
		u := strings.ToUpper(q)
		if !strings.Contains(u, "DELETE") || !strings.Contains(q, "expires_at <= $1") {
			t.Fatalf("batch %d must DELETE by expires_at cutoff, got: %s", i, q)
		}
		if !strings.Contains(u, "ORDER BY") || !strings.Contains(q, "LIMIT $2") {
			t.Fatalf("batch %d must bound the batch with ORDER BY/LIMIT, got: %s", i, q)
		}
		if len(fake.args[i]) != 2 {
			t.Fatalf("batch %d args = %v, want [cutoff batch]", i, fake.args[i])
		}
		cutoff, ok := fake.args[i][0].(time.Time)
		if !ok || !cutoff.Equal(wantCutoff) {
			t.Fatalf("batch %d cutoff = %v, want exactly %v", i, fake.args[i][0], wantCutoff)
		}
		if fake.args[i][1] != 2 {
			t.Fatalf("batch %d limit = %v, want 2", i, fake.args[i][1])
		}
		if strings.Contains(q, "purge-me") {
			t.Fatalf("batch %d interpolates a hash into SQL text: %s", i, q)
		}
	}
}

func TestLoginStatePurge_ZeroRowsIsSuccess(t *testing.T) {
	t.Parallel()
	fake := &purgeFakeDB{scripts: []purgeScript{{rows: 0}}}
	p := NewLoginStatePurger(fake, nil)
	got, err := p.Purge(context.Background(), time.Now().UTC(), LoginStateRetention, 10)
	if err != nil {
		t.Fatalf("empty purge: %v", err)
	}
	if got != 0 {
		t.Fatalf("empty purge deleted = %d, want 0", got)
	}
}

func TestLoginStatePurge_MidLoopErrorKeepsPartialCount(t *testing.T) {
	t.Parallel()
	boom := errors.New("sessionstore test: batch two fails")
	fake := &purgeFakeDB{scripts: []purgeScript{{rows: 2}, {execErr: boom}}}
	p := NewLoginStatePurger(fake, nil)
	got, err := p.Purge(context.Background(), time.Now().UTC(), LoginStateRetention, 2)
	if !errors.Is(err, boom) {
		t.Fatalf("purge error = %v, want the batch failure", err)
	}
	if got != 2 {
		t.Fatalf("purge deleted = %d, want the 2 rows from the first batch", got)
	}
}

func TestLoginStatePurge_RowsAffectedErrorPropagates(t *testing.T) {
	t.Parallel()
	boom := errors.New("sessionstore test: rows affected fails")
	fake := &purgeFakeDB{scripts: []purgeScript{{affectedErr: boom}}}
	p := NewLoginStatePurger(fake, nil)
	if _, err := p.Purge(context.Background(), time.Now().UTC(), LoginStateRetention, 10); !errors.Is(err, boom) {
		t.Fatalf("purge error = %v, want the RowsAffected failure", err)
	}
}

func TestLoginStatePurge_CancelledBetweenBatchesStops(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake := &purgeFakeDB{
		scripts: []purgeScript{{rows: 2}, {rows: 2}},
		onExec:  func(call int) { cancel() },
	}
	p := NewLoginStatePurger(fake, nil)
	got, err := p.Purge(ctx, time.Now().UTC(), LoginStateRetention, 2)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("purge error = %v, want context.Canceled", err)
	}
	if got != 2 {
		t.Fatalf("purge deleted = %d, want the first batch only", got)
	}
	if len(fake.queries) != 1 {
		t.Fatalf("cancelled purge batches = %d, want 1", len(fake.queries))
	}
}
