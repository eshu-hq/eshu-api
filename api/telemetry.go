// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// histogram is a cumulative-bucket histogram over durationBuckets plus
// the implicit +Inf bucket, with a seconds sum.
type histogram struct {
	buckets []uint64
	sum     float64
	count   uint64
}

// observe files one seconds sample into the cumulative buckets.
func (h *histogram) observe(seconds float64) {
	h.count++
	h.sum += seconds
	for i, bound := range durationBuckets {
		if seconds <= bound {
			h.buckets[i]++
		}
	}
	h.buckets[len(durationBuckets)]++
}

// snapshot copies the histogram for lock-free test assertions.
func (h *histogram) snapshot() histSnapshot {
	buckets := make([]uint64, len(h.buckets))
	copy(buckets, h.buckets)
	return histSnapshot{count: h.count, sum: h.sum, buckets: buckets}
}

// histSnapshot is the assertion view of one histogram.
type histSnapshot struct {
	count   uint64
	sum     float64
	buckets []uint64
}

// telemetrySnapshot is the assertion view of the whole registry.
type telemetrySnapshot struct {
	requests           map[string]map[string]uint64
	durations          map[string]histSnapshot
	dispatches         map[string]map[string]map[string]uint64
	coreLatencies      map[string]map[string]histSnapshot
	authorizer         map[string]map[string]uint64
	readiness          map[string]map[string]uint64
	auditDrops         uint64
	auditDroppedEvents uint64
}

// Telemetry is the in-process destination signal registry behind
// Satellite Leaf E (Refs eshu-hq/eshu#9669, part of #6707). It answers
// the operator question "which family, native or relay, denied or
// 503, core slow or dest slow" from dest signals alone:
//
//   - requests_total by family and status class, plus the
//     request-duration histogram by family (dest-side latency);
//   - dispatch_total by family, lane (native vs relay), and outcome,
//     plus the core-latency histogram by family and lane, so a local
//     permanent 503 reads apart from a relayed retryable 503;
//   - authorizer_total by family and allow/deny decision, so the
//     unreachable deny branch stays observable;
//   - readiness_total by result (ready vs not_ready) and closed
//     failure reason, so an operator can see why /readyz failed
//     without reading the probe body;
//   - audit_drops_total wired to the Leaf A drop-observer contract;
//   - one structured log line per dispatch request keyed on the
//     correlation ID the auth gate echoes-or-mints.
//
// Labels are closed vocabularies only: family comes from the bounded
// family table, never from raw path params. The zero value is not
// usable; build with NewTelemetry. It is safe for concurrent use.
type Telemetry struct {
	logger *slog.Logger

	mu                 sync.Mutex
	requests           map[string]map[string]uint64
	durations          map[string]*histogram
	dispatches         map[string]map[string]map[string]uint64
	coreLatencies      map[string]map[string]*histogram
	authorizer         map[string]map[string]uint64
	readiness          map[string]map[string]uint64
	auditDrops         uint64
	auditDroppedEvents uint64
}

// NewTelemetry builds a registry that logs request lines through
// logger. A nil logger disables log emission; metrics still record.
func NewTelemetry(logger *slog.Logger) *Telemetry {
	return &Telemetry{
		logger:        logger,
		requests:      make(map[string]map[string]uint64),
		durations:     make(map[string]*histogram),
		dispatches:    make(map[string]map[string]map[string]uint64),
		coreLatencies: make(map[string]map[string]*histogram),
		authorizer:    make(map[string]map[string]uint64),
		readiness:     make(map[string]map[string]uint64),
	}
}

// ObserveAuditDrop records one swallowed audit-append failure. Its
// shape mirrors the Leaf A observer contract in eshu
// (go/internal/query/governance_audit_admission.go):
// GovernanceAuditDropObserver func(ctx context.Context, err error,
// eventCount int). A satellite-native runtime wires this method value
// to ObservedGovernanceAuditAppender.OnDrop so dropped audits count
// here; dest emits no audits today, so the counter rests at zero until
// that port lands.
func (t *Telemetry) ObserveAuditDrop(_ context.Context, _ error, eventCount int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.auditDrops++
	t.auditDroppedEvents += uint64(eventCount)
}

// observeRequest files one finished dispatch request.
func (t *Telemetry) observeRequest(family, class string, duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	classes, ok := t.requests[family]
	if !ok {
		classes = make(map[string]uint64)
		t.requests[family] = classes
	}
	classes[class]++
	hist, ok := t.durations[family]
	if !ok {
		hist = &histogram{buckets: make([]uint64, len(durationBuckets)+1)}
		t.durations[family] = hist
	}
	hist.observe(duration.Seconds())
}

// observeDispatch files one dispatch step: the lane that served it,
// the closed outcome, and the core-side latency in seconds.
func (t *Telemetry) observeDispatch(family, via, outcome string, coreLatency time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	lanes, ok := t.dispatches[family]
	if !ok {
		lanes = make(map[string]map[string]uint64)
		t.dispatches[family] = lanes
	}
	outcomes, ok := lanes[via]
	if !ok {
		outcomes = make(map[string]uint64)
		lanes[via] = outcomes
	}
	outcomes[outcome]++
	lanesHist, ok := t.coreLatencies[family]
	if !ok {
		lanesHist = make(map[string]*histogram)
		t.coreLatencies[family] = lanesHist
	}
	hist, ok := lanesHist[via]
	if !ok {
		hist = &histogram{buckets: make([]uint64, len(durationBuckets)+1)}
		lanesHist[via] = hist
	}
	hist.observe(coreLatency.Seconds())
}

// observeAuthorizer files one family authorizer judgment.
func (t *Telemetry) observeAuthorizer(family, decision string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	decisions, ok := t.authorizer[family]
	if !ok {
		decisions = make(map[string]uint64)
		t.authorizer[family] = decisions
	}
	decisions[decision]++
}

// observeReadiness files one /readyz verdict: the closed result
// (ready vs not_ready, see probes.go) with the closed failure reason.
// Probes never pass through the instrumented chain, so readiness is
// the only signal that observes them; both labels stay in the closed
// vocabulary, keeping /metrics cardinality bounded.
func (t *Telemetry) observeReadiness(result, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	reasons, ok := t.readiness[result]
	if !ok {
		reasons = make(map[string]uint64)
		t.readiness[result] = reasons
	}
	reasons[reason]++
}

// snapshot copies the registry for test assertions.
func (t *Telemetry) snapshot() telemetrySnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	snap := telemetrySnapshot{
		requests:      make(map[string]map[string]uint64),
		durations:     make(map[string]histSnapshot),
		dispatches:    make(map[string]map[string]map[string]uint64),
		coreLatencies: make(map[string]map[string]histSnapshot),
		authorizer:    make(map[string]map[string]uint64),
		readiness:     make(map[string]map[string]uint64),
	}
	for family, classes := range t.requests {
		copied := make(map[string]uint64, len(classes))
		for class, count := range classes {
			copied[class] = count
		}
		snap.requests[family] = copied
	}
	for family, hist := range t.durations {
		snap.durations[family] = hist.snapshot()
	}
	for family, lanes := range t.dispatches {
		copiedLanes := make(map[string]map[string]uint64, len(lanes))
		for via, outcomes := range lanes {
			copied := make(map[string]uint64, len(outcomes))
			for outcome, count := range outcomes {
				copied[outcome] = count
			}
			copiedLanes[via] = copied
		}
		snap.dispatches[family] = copiedLanes
	}
	for family, lanes := range t.coreLatencies {
		copied := make(map[string]histSnapshot, len(lanes))
		for via, hist := range lanes {
			copied[via] = hist.snapshot()
		}
		snap.coreLatencies[family] = copied
	}
	for family, decisions := range t.authorizer {
		copied := make(map[string]uint64, len(decisions))
		for decision, count := range decisions {
			copied[decision] = count
		}
		snap.authorizer[family] = copied
	}
	for result, reasons := range t.readiness {
		copied := make(map[string]uint64, len(reasons))
		for reason, count := range reasons {
			copied[reason] = count
		}
		snap.readiness[result] = copied
	}
	snap.auditDrops = t.auditDrops
	snap.auditDroppedEvents = t.auditDroppedEvents
	return snap
}
