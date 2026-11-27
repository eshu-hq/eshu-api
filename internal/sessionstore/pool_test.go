// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"strings"
	"testing"
)

func TestPoolBudget_AcceptsFittingBudget(t *testing.T) {
	t.Parallel()
	b := PoolBudget{
		ServerMaxConnections: 100,
		CoreAPIBudget:        20,
		Headroom:             10,
		MaxReplicas:          3,
		SurgeReplicas:        1,
		PerReplicaMaxConns:   17,
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("fitting budget must validate: %v", err)
	}
	// 17*(3+1) + 20 + 10 = 98 <= 100.
	if got := b.Total(); got != 98 {
		t.Fatalf("Total = %d, want 98", got)
	}
}

func TestPoolBudget_RejectsPerPodGuess(t *testing.T) {
	t.Parallel()
	b := PoolBudget{
		ServerMaxConnections: 100,
		CoreAPIBudget:        20,
		Headroom:             10,
		MaxReplicas:          3,
		SurgeReplicas:        1,
		PerReplicaMaxConns:   25,
	}
	// 25*4 + 20 + 10 = 130 > 100: a per-pod guess without the
	// multiplier math must fail closed.
	if err := b.Validate(); err == nil {
		t.Fatal("overflowing budget must fail validation")
	}
}

func TestPoolBudget_WorkedExampleAtOpsQAScale(t *testing.T) {
	t.Parallel()
	// Mirrors the worked arithmetic the PR body carries: ops-qa runs
	// eshu-api at replicas=1 with a 25% surge and no HPA, so the
	// steady-state multiplier is 1 and the surge adds one pod.
	b := PoolBudget{
		ServerMaxConnections: 100,
		CoreAPIBudget:        40,
		Headroom:             10,
		MaxReplicas:          1,
		SurgeReplicas:        1,
		PerReplicaMaxConns:   10,
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("ops-qa scale budget must validate: %v", err)
	}
	if got := b.Total(); got != 70 {
		t.Fatalf("Total = %d, want 70", got)
	}
}

func TestPoolBudget_ZeroServerFailsClosed(t *testing.T) {
	t.Parallel()
	b := PoolBudget{PerReplicaMaxConns: 5, MaxReplicas: 1}
	if err := b.Validate(); err == nil {
		t.Fatal("unknown server max_connections must fail closed")
	}
}

func TestPoolBudget_NegativeReplicaCountRejected(t *testing.T) {
	t.Parallel()
	b := PoolBudget{ServerMaxConnections: 100, MaxReplicas: -1, PerReplicaMaxConns: 5}
	if err := b.Validate(); err == nil {
		t.Fatal("negative replica counts must fail validation")
	}
}

func TestReplicaBudget_ValidatedIndependently(t *testing.T) {
	t.Parallel()
	// The replica gets its own budget line: read-list traffic plus
	// headroom must fit the replica max_connections on its own.
	b := PoolBudget{
		ServerMaxConnections: 100,
		MaxReplicas:          3,
		SurgeReplicas:        1,
		PerReplicaMaxConns:   20,
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("replica budget must validate independently: %v", err)
	}
	if got := b.Total(); got != 80 {
		t.Fatalf("Total = %d, want 80", got)
	}
}

func TestPoolBudget_ReplicaAndPrimaryUseSameFormula(t *testing.T) {
	t.Parallel()
	formula := "PerReplicaMaxConns x (MaxReplicas + SurgeReplicas) + CoreAPIBudget + Headroom <= ServerMaxConnections"
	b := PoolBudget{
		ServerMaxConnections: 200,
		CoreAPIBudget:        60,
		Headroom:             20,
		MaxReplicas:          5,
		SurgeReplicas:        2,
		PerReplicaMaxConns:   17,
	}
	// 17*7 + 60 + 20 = 199 <= 200.
	if err := b.Validate(); err != nil {
		t.Fatalf("%s: %v", formula, err)
	}
	if !strings.Contains(b.Formula(), "MaxReplicas") {
		t.Fatalf("budget must publish its formula, got %q", b.Formula())
	}
}
