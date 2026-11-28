// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

// Fixture PostgreSQL driver registration for TestFixture_* suites.
// The pgx stdlib driver lets the fixture tests open ESHU_TEST_PG_DSN
// through database/sql; without a registered driver they skip and the
// purge/migration SQL stays unproven against a live database.
import _ "github.com/jackc/pgx/v5/stdlib"
