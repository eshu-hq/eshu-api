// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"fmt"
)

// PoolBudget pins the global connection arithmetic from ADR-0002: the
// chart sets a per-replica ceiling such that the whole fleet plus the
// standing core-api consumers and headroom fits the server's
// max_connections. A bare per-pod number is not a budget; Validate
// rejects anything that does not carry the multiplier math.
//
// The same formula validates the primary line (read-write pool plus
// core-api budget) and the replica line (read-list pool): only the
// inputs differ.
type PoolBudget struct {
	// ServerMaxConnections is the PostgreSQL max_connections of the
	// server this line targets. Zero or negative fails closed:
	// without a measured ceiling no budget is safe.
	ServerMaxConnections int
	// CoreAPIBudget reserves connections for the standing core-api
	// consumers sharing the server. Zero on the replica line.
	CoreAPIBudget int
	// Headroom reserves connections for ops (migrations, psql).
	Headroom int
	// MaxReplicas is the maximum steady-state eshu-api replica
	// count the chart allows (the HPA ceiling when autoscaling
	// lands; the Deployment replica count until then).
	MaxReplicas int
	// SurgeReplicas is the extra pods a rollout may add above
	// MaxReplicas (maxSurge).
	SurgeReplicas int
	// PerReplicaMaxConns is the per-pod pool ceiling being sized.
	PerReplicaMaxConns int
}

// Formula publishes the arithmetic Validate enforces, so chart
// comments and the PR body can quote it instead of a magic number.
func (b PoolBudget) Formula() string {
	return "PerReplicaMaxConns x (MaxReplicas + SurgeReplicas) + CoreAPIBudget + Headroom <= ServerMaxConnections"
}

// Total is the fleet-wide connection demand this budget implies.
func (b PoolBudget) Total() int {
	return b.PerReplicaMaxConns*(b.MaxReplicas+b.SurgeReplicas) + b.CoreAPIBudget + b.Headroom
}

// Validate fails closed on any budget that overflows the server
// ceiling or rests on unmeasured inputs.
func (b PoolBudget) Validate() error {
	if b.ServerMaxConnections <= 0 {
		return fmt.Errorf("sessionstore: server max_connections unknown, refusing to size pools")
	}
	if b.MaxReplicas < 0 || b.SurgeReplicas < 0 {
		return fmt.Errorf("sessionstore: negative replica counts (max=%d surge=%d)", b.MaxReplicas, b.SurgeReplicas)
	}
	if b.PerReplicaMaxConns <= 0 {
		return fmt.Errorf("sessionstore: per-replica ceiling must be positive")
	}
	if b.CoreAPIBudget < 0 || b.Headroom < 0 {
		return fmt.Errorf("sessionstore: negative shared budget entry (core=%d headroom=%d) would shrink demand and void the formula",
			b.CoreAPIBudget, b.Headroom)
	}
	if total := b.Total(); total > b.ServerMaxConnections {
		return fmt.Errorf("sessionstore: pool demand %d exceeds server max_connections %d (%s)",
			total, b.ServerMaxConnections, b.Formula())
	}
	return nil
}
