package engine

import (
	"runtime"
	"sort"
	"time"
)

type StageDiagnostics struct {
	Stage       string  `json:"stage"`
	ModelSHA256 string  `json:"model_sha256"`
	Runs        uint64  `json:"runs"`
	RunSeconds  float64 `json:"run_seconds"`
}

type Diagnostics struct {
	RuntimeVersion   string             `json:"runtime_version"`
	Platform         string             `json:"platform"`
	Threads          int                `json:"threads"`
	MaxInFlight      int                `json:"max_inflight"`
	Active           int                `json:"active"`
	QueueWaitSeconds float64            `json:"queue_wait_seconds"`
	BatchRequests    uint64             `json:"batch_requests"`
	Closed           bool               `json:"closed"`
	Stages           []StageDiagnostics `json:"stages"`
}

// Diagnostics returns runtime information and stage timings, including after Close.
func (e *Engine) Diagnostics() Diagnostics {
	if e == nil {
		return Diagnostics{Closed: true, Stages: []StageDiagnostics{}}
	}
	if e.slots == nil {
		if e.gate == nil {
			return Diagnostics{Closed: true, Stages: []StageDiagnostics{}}
		}
		e.slots = e.gate
	}
	e.stateMu.Lock()
	closed := e.closed
	e.stateMu.Unlock()
	e.resourceMu.RLock()
	defer e.resourceMu.RUnlock()
	out := Diagnostics{RuntimeVersion: e.runtimeVersion, Platform: runtime.GOOS + "/" + runtime.GOARCH, Threads: e.threads, MaxInFlight: e.maxInFlight, Active: int(e.activeCount.Load()), QueueWaitSeconds: float64(e.queueWaitNanos.Load()) / 1e9, BatchRequests: e.batchRequests.Load(), Closed: closed, Stages: []StageDiagnostics{}}
	networks := []*network{e.detector, e.classifier}
	e.recognizerMu.Lock()
	for _, r := range e.recognizers {
		networks = append(networks, r.model)
	}
	e.recognizerMu.Unlock()
	for _, n := range networks {
		if n != nil {
			out.Stages = append(out.Stages, n.snapshotDiagnostics())
		}
	}
	sort.Slice(out.Stages, func(i, j int) bool { return out.Stages[i].Stage < out.Stages[j].Stage })
	return out
}

func (n *network) recordRun(start time.Time) {
	n.diagnosticsMu.Lock()
	defer n.diagnosticsMu.Unlock()
	n.diagnostics.Runs++
	n.diagnostics.RunSeconds += time.Since(start).Seconds()
}
