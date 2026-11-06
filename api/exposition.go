// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// histRow is one ordered histogram row for exposition.
type histRow struct {
	labels string
	hist   histSnapshot
}

// sortedKeys orders one map level for deterministic exposition.
func sortedKeys[V any](rows map[string]V) []string {
	names := make([]string, 0, len(rows))
	for name := range rows {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// render exposes the registry in Prometheus text exposition format
// (0.0.4): one HELP/TYPE pair per instrument, sorted family rows, and
// cumulative histogram buckets with the implicit +Inf bucket. Label
// values come from closed vocabularies only, so no escaping is needed.
func (t *Telemetry) render() string {
	snap := t.snapshot()
	var out strings.Builder

	writeCounter := func(name, help string, rows []string) {
		out.WriteString("# HELP " + name + " " + help + "\n")
		out.WriteString("# TYPE " + name + " counter\n")
		for _, row := range rows {
			out.WriteString(row + "\n")
		}
	}
	// writeHistogram renders one histogram instrument from ordered
	// (labels, snapshot) rows.
	writeHistogram := func(name, help string, rows []histRow) {
		out.WriteString("# HELP " + name + " " + help + "\n")
		out.WriteString("# TYPE " + name + " histogram\n")
		for _, row := range rows {
			for i, bound := range durationBuckets {
				out.WriteString(name + `_bucket{` + row.labels + `,le="` +
					strconv.FormatFloat(bound, 'g', -1, 64) + `"} ` +
					strconv.FormatUint(row.hist.buckets[i], 10) + "\n")
			}
			out.WriteString(name + `_bucket{` + row.labels + `,le="+Inf"} ` +
				strconv.FormatUint(row.hist.count, 10) + "\n")
			out.WriteString(name + `_sum{` + row.labels + `} ` +
				strconv.FormatFloat(row.hist.sum, 'f', -1, 64) + "\n")
			out.WriteString(name + `_count{` + row.labels + `} ` +
				strconv.FormatUint(row.hist.count, 10) + "\n")
		}
	}

	var requestRows []string
	for _, family := range sortedKeys(snap.requests) {
		classes := make([]string, 0, len(snap.requests[family]))
		for class := range snap.requests[family] {
			classes = append(classes, class)
		}
		sort.Strings(classes)
		for _, class := range classes {
			requestRows = append(requestRows, fmt.Sprintf(
				`eshu_api_requests_total{family="%s",class="%s"} %d`,
				family, class, snap.requests[family][class]))
		}
	}
	writeCounter("eshu_api_requests_total",
		"Authenticated dispatch requests by route family and status class.", requestRows)

	var durationRows []histRow
	for _, family := range sortedKeys(snap.durations) {
		if hist, ok := snap.durations[family]; ok {
			durationRows = append(durationRows, histRow{labels: `family="` + family + `"`, hist: hist})
		}
	}
	// Families are already ordered: families() sorts.
	writeHistogram("eshu_api_request_duration_seconds",
		"End-to-end dispatch request latency in seconds by route family.", durationRows)

	var dispatchRows []string
	for _, family := range sortedKeys(snap.dispatches) {
		lanes := make([]string, 0, len(snap.dispatches[family]))
		for via := range snap.dispatches[family] {
			lanes = append(lanes, via)
		}
		sort.Strings(lanes)
		for _, via := range lanes {
			outcomes := make([]string, 0, len(snap.dispatches[family][via]))
			for outcome := range snap.dispatches[family][via] {
				outcomes = append(outcomes, outcome)
			}
			sort.Strings(outcomes)
			for _, outcome := range outcomes {
				dispatchRows = append(dispatchRows, fmt.Sprintf(
					`eshu_api_dispatch_total{family="%s",via="%s",outcome="%s"} %d`,
					family, via, outcome, snap.dispatches[family][via][outcome]))
			}
		}
	}
	writeCounter("eshu_api_dispatch_total",
		"Dispatch steps by route family, lane (native vs relay), and outcome.", dispatchRows)

	var latencyRows []histRow
	for _, family := range sortedKeys(snap.coreLatencies) {
		lanes := make([]string, 0, len(snap.coreLatencies[family]))
		for via := range snap.coreLatencies[family] {
			lanes = append(lanes, via)
		}
		sort.Strings(lanes)
		for _, via := range lanes {
			latencyRows = append(latencyRows, histRow{
				labels: `family="` + family + `",via="` + via + `"`,
				hist:   snap.coreLatencies[family][via],
			})
		}
	}
	writeHistogram("eshu_api_core_latency_seconds",
		"Core-side latency in seconds by route family and lane.", latencyRows)

	var authorizerRows []string
	for _, family := range sortedKeys(snap.authorizer) {
		decisions := []string{decisionAllow, decisionDeny}
		for _, decision := range decisions {
			if count, ok := snap.authorizer[family][decision]; ok {
				authorizerRows = append(authorizerRows, fmt.Sprintf(
					`eshu_api_authorizer_total{family="%s",decision="%s"} %d`,
					family, decision, count))
			}
		}
	}
	writeCounter("eshu_api_authorizer_total",
		"Family authorizer judgments by route family and decision.", authorizerRows)

	writeCounter("eshu_api_audit_drops_total",
		"Swallowed audit-append failures observed through the Leaf A drop-observer hook.",
		[]string{"eshu_api_audit_drops_total " + strconv.FormatUint(snap.auditDrops, 10)})
	writeCounter("eshu_api_audit_dropped_events_total",
		"Audit events lost to swallowed audit-append failures.",
		[]string{"eshu_api_audit_dropped_events_total " + strconv.FormatUint(snap.auditDroppedEvents, 10)})

	return out.String()
}

// metrics answers GET /metrics with the registry in Prometheus text
// exposition format. It is public like the probes: it carries only
// closed-vocabulary counters, never credentials, paths, or bodies.
func (s *Server) metrics(w http.ResponseWriter, _ *http.Request) {
	body := ""
	if s.telemetry != nil {
		body = s.telemetry.render()
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}
