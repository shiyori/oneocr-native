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
	RuntimeVersion string             `json:"runtime_version"`
	Platform       string             `json:"platform"`
	Threads        int                `json:"threads"`
	Closed         bool               `json:"closed"`
	Stages         []StageDiagnostics `json:"stages"`
}

// Diagnostics returns runtime information and stage timings, including after Close.
func (e *Engine) Diagnostics() Diagnostics {
	if e == nil || e.gate == nil {
		return Diagnostics{Closed: true, Stages: []StageDiagnostics{}}
	}
	e.gate <- struct{}{}
	defer e.unlock()
	out := Diagnostics{RuntimeVersion: e.runtimeVersion, Platform: runtime.GOOS + "/" + runtime.GOARCH, Threads: e.threads, Closed: e.closed, Stages: []StageDiagnostics{}}
	networks := []*network{e.detector, e.classifier}
	for _, r := range e.recognizers {
		networks = append(networks, r.model)
	}
	for _, n := range networks {
		if n != nil {
			out.Stages = append(out.Stages, n.diagnostics)
		}
	}
	sort.Slice(out.Stages, func(i, j int) bool { return out.Stages[i].Stage < out.Stages[j].Stage })
	return out
}

func (n *network) recordRun(start time.Time) {
	n.diagnostics.Runs++
	n.diagnostics.RunSeconds += time.Since(start).Seconds()
}
