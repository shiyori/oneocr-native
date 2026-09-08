package oneocr

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	ort "github.com/yalue/onnxruntime_go"
)

// Backend selects an ORT execution provider, not a guarantee of GPU execution.
type Backend string

const (
	BackendCPU      Backend = "cpu"
	BackendCoreML   Backend = "coreml"
	BackendCUDA     Backend = "cuda"
	BackendDirectML Backend = "directml"
)

type FallbackPolicy string

const (
	FallbackCPU   FallbackPolicy = "cpu"
	FallbackError FallbackPolicy = "error"
)

type ProviderUsage struct {
	KernelEvents       uint64  `json:"kernel_events"`
	KernelMicroseconds float64 `json:"kernel_microseconds"`
}

// StageDiagnostics distinguishes session registration from profiled execution.
// A CoreML event alone does not identify whether CoreML used CPU, GPU or ANE.
type StageDiagnostics struct {
	Stage             string                   `json:"stage"`
	Requested         Backend                  `json:"requested"`
	Registered        Backend                  `json:"registered"`
	ModelSHA256       string                   `json:"model_sha256"`
	Recipe            string                   `json:"recipe,omitempty"`
	Experimental      bool                     `json:"experimental"`
	CompactOutput     bool                     `json:"compact_output"`
	FallbackReason    string                   `json:"fallback_reason,omitempty"`
	Runs              uint64                   `json:"runs"`
	RunSeconds        float64                  `json:"run_seconds"`
	ExecutionMeasured bool                     `json:"execution_measured"`
	Providers         map[string]ProviderUsage `json:"providers,omitempty"`
	ProfileFiles      []string                 `json:"profile_files,omitempty"`
	ProfileError      string                   `json:"profile_error,omitempty"`
	ShapeKey          string                   `json:"shape_key,omitempty"`
	ShapeCacheHits    uint64                   `json:"shape_cache_hits,omitempty"`
	ShapeCacheMisses  uint64                   `json:"shape_cache_misses,omitempty"`
	ShapeEvictions    uint64                   `json:"shape_evictions,omitempty"`
	ShapeSessions     []StageDiagnostics       `json:"shape_sessions,omitempty"`
}
type Diagnostics struct {
	RuntimeVersion string             `json:"runtime_version"`
	Platform       string             `json:"platform"`
	Backend        Backend            `json:"backend"`
	DeviceID       int                `json:"device_id"`
	Threads        int                `json:"threads"`
	Closed         bool               `json:"closed"`
	Stages         []StageDiagnostics `json:"stages"`
}

func validBackend(b Backend) bool {
	return b == BackendCPU || b == BackendCoreML || b == BackendCUDA || b == BackendDirectML
}
func normalizeBackendConfig(c *Config) error {
	if c.ShapeCacheSize < 0 || c.ShapeCacheSize > 4 {
		return fmt.Errorf("oneocr: shape_cache_size must be 0..4")
	}
	if c.Backend == "" {
		c.Backend = BackendCPU
	}
	if c.Fallback == "" {
		c.Fallback = FallbackCPU
	}
	if c.CoreMLComputeUnits == "" {
		c.CoreMLComputeUnits = "ALL"
	}
	if !validBackend(c.Backend) || c.DeviceID < 0 || (c.Fallback != FallbackCPU && c.Fallback != FallbackError) {
		return fmt.Errorf("oneocr: invalid backend, device_id or fallback policy")
	}
	switch c.CoreMLComputeUnits {
	case "ALL", "CPUOnly", "CPUAndGPU", "CPUAndNeuralEngine":
	default:
		return fmt.Errorf("oneocr: invalid CoreML compute units")
	}
	c.StageBackends = cloneStageBackends(c.StageBackends)
	for stage, b := range c.StageBackends {
		if !validBackend(b) || !validStage(stage) {
			return fmt.Errorf("oneocr: invalid stage backend %q=%q", stage, b)
		}
	}
	c.CharacterClasses = append([]CharacterClass(nil), c.CharacterClasses...)
	if _, err := characterClassSet(c.CharacterClasses); err != nil {
		return err
	}
	if c.ProfilingDir != "" {
		path, err := filepath.Abs(c.ProfilingDir)
		if err != nil {
			return err
		}
		c.ProfilingDir = path
		if err = os.MkdirAll(path, 0700); err != nil {
			return err
		}
	}
	return nil
}
func validStage(s string) bool {
	if s == "detector" || s == "classifier" {
		return true
	}
	for _, script := range scripts {
		if s == "recognizer/"+script {
			return true
		}
	}
	return false
}
func cloneStageBackends(m map[string]Backend) map[string]Backend {
	out := make(map[string]Backend, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
func (c Config) stageBackend(stage string) Backend {
	if b, ok := c.StageBackends[stage]; ok {
		return b
	}
	return c.Backend
}
func backendPlatform(b Backend) error {
	switch b {
	case BackendCoreML:
		if runtime.GOOS != "darwin" {
			return fmt.Errorf("CoreML requires an Apple runtime")
		}
	case BackendCUDA:
		if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
			return fmt.Errorf("CUDA requires a supported NVIDIA runtime on Windows/Linux")
		}
	case BackendDirectML:
		if runtime.GOOS != "windows" {
			return fmt.Errorf("DirectML requires Windows with a DirectX 12 device")
		}
	}
	return nil
}

func sessionOptions(config Config, backend Backend, modelHash, stage string) (*ort.SessionOptions, string, error) {
	options, err := ort.NewSessionOptions()
	if err != nil {
		return nil, "", err
	}
	success := false
	defer func() {
		if !success {
			options.Destroy()
		}
	}()
	settings := []func() error{
		func() error { return options.SetIntraOpNumThreads(config.Threads) },
		func() error { return options.SetInterOpNumThreads(1) },
		func() error { return options.SetExecutionMode(ort.ExecutionModeSequential) },
		func() error { return options.SetLogSeverityLevel(3) },
		func() error { return options.AddSessionConfigEntry("session.intra_op.allow_spinning", "0") },
		func() error { return options.AddSessionConfigEntry("session.inter_op.allow_spinning", "0") },
	}
	for _, set := range settings {
		if err = set(); err != nil {
			return nil, "", err
		}
	}
	if err = backendPlatform(backend); err != nil {
		return nil, "", err
	}
	switch backend {
	case BackendCoreML:
		cache := config.CacheDir
		if cache == "" {
			cache, err = os.UserCacheDir()
			if err != nil {
				return nil, "", err
			}
			cache = filepath.Join(cache, "oneocr")
		}
		// In-memory ORT cache keys may only hash graph names, so isolate by the
		// complete model bytes, ORT version, platform and provider options here.
		cache, err = filepath.Abs(filepath.Join(cache, "coreml", ort.GetVersion(), runtime.GOOS+"-"+runtime.GOARCH, config.CoreMLComputeUnits, modelHash))
		if err != nil {
			return nil, "", err
		}
		if err = os.MkdirAll(cache, 0700); err != nil {
			return nil, "", err
		}
		err = options.AppendExecutionProviderCoreMLV2(map[string]string{"ModelFormat": "MLProgram", "MLComputeUnits": config.CoreMLComputeUnits, "RequireStaticInputShapes": "0", "EnableOnSubgraphs": "0", "AllowLowPrecisionAccumulationOnGPU": "0", "ModelCacheDirectory": cache})
	case BackendCUDA:
		var cuda *ort.CUDAProviderOptions
		cuda, err = ort.NewCUDAProviderOptions()
		if err == nil {
			defer cuda.Destroy()
			err = cuda.Update(map[string]string{"device_id": strconv.Itoa(config.DeviceID), "do_copy_in_default_stream": "1", "use_tf32": "0", "cudnn_conv_algo_search": "HEURISTIC"})
			if err == nil {
				err = options.AppendExecutionProviderCUDA(cuda)
			}
		}
	case BackendDirectML:
		if err = options.SetMemPattern(false); err == nil {
			err = options.AppendExecutionProviderDirectML(config.DeviceID)
		}
	}
	if err != nil {
		return nil, "", err
	}
	profileDir := ""
	if config.ProfilingDir != "" {
		profileDir, err = os.MkdirTemp(config.ProfilingDir, strings.ReplaceAll(stage, "/", "-")+"-")
		if err != nil {
			return nil, "", err
		}
		if err = options.EnableProfiling(filepath.Join(profileDir, "ort")); err != nil {
			return nil, "", err
		}
	}
	success = true
	return options, profileDir, nil
}

func (n *network) createSession(model []byte, backend Backend, outputs []string) error {
	hash := fmt.Sprintf("%x", sha256.Sum256(model))
	options, profileDir, err := sessionOptions(n.config, backend, hash, n.diagnostics.Stage)
	if err != nil {
		return err
	}
	defer options.Destroy()
	inputs := append([]string(nil), n.inputs...)
	compact := len(outputs) == 2 && outputs[0] == "token_ids" && outputs[1] == "invalid"
	if compact {
		inputs = append(inputs, "allowed_tokens")
	}
	s, err := ort.NewDynamicAdvancedSessionWithONNXData(model, inputs, outputs, options)
	if err != nil {
		if profileDir != "" {
			n.collectProfile(profileDir)
		}
		return err
	}
	n.session = s
	n.outputs = append([]string(nil), outputs...)
	n.profileDir = profileDir
	n.diagnostics.Registered = backend
	n.diagnostics.ModelSHA256 = hash
	n.diagnostics.CompactOutput = compact
	return nil
}

// collectProfile streams finalized ORT events rather than retaining a potentially
// large trace. It is called after Destroy, which flushes profiling in ORT 1.29.
func (n *network) collectProfile(directory string) {
	paths, err := filepath.Glob(filepath.Join(directory, "ort_*.json"))
	if err != nil {
		n.diagnostics.ProfileError = err.Error()
		return
	}
	for _, path := range paths {
		n.diagnostics.ProfileFiles = append(n.diagnostics.ProfileFiles, path)
		usage, measured, err := readProviderProfile(path)
		if err != nil {
			n.diagnostics.ProfileError = err.Error()
			continue
		}
		if n.diagnostics.Providers == nil {
			n.diagnostics.Providers = map[string]ProviderUsage{}
		}
		for provider, u := range usage {
			old := n.diagnostics.Providers[provider]
			old.KernelEvents += u.KernelEvents
			old.KernelMicroseconds += u.KernelMicroseconds
			n.diagnostics.Providers[provider] = old
		}
		n.diagnostics.ExecutionMeasured = n.diagnostics.ExecutionMeasured || measured
	}
}
func readProviderProfile(path string) (map[string]ProviderUsage, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 512*1024*1024))
	tok, err := d.Token()
	if err != nil || tok != json.Delim('[') {
		return nil, false, fmt.Errorf("invalid ORT profile: %s", path)
	}
	out := map[string]ProviderUsage{}
	measured := false
	for d.More() {
		var event struct {
			Name     string  `json:"name"`
			Duration float64 `json:"dur"`
			Args     struct {
				Provider string `json:"provider"`
			} `json:"args"`
		}
		if err = d.Decode(&event); err != nil {
			return nil, false, err
		}
		if event.Args.Provider != "" && strings.HasSuffix(event.Name, "_kernel_time") {
			u := out[event.Args.Provider]
			u.KernelEvents++
			u.KernelMicroseconds += event.Duration
			out[event.Args.Provider] = u
			measured = true
		}
	}
	if _, err = d.Token(); err != nil {
		return nil, false, err
	}
	return out, measured, nil
}

// Diagnostics is safe during use and after Close. Before profiling is finalized,
// Registered reports session setup only and ExecutionMeasured remains false.
func (e *Engine) Diagnostics() Diagnostics {
	if e == nil || e.gate == nil {
		return Diagnostics{Closed: true, Stages: []StageDiagnostics{}}
	}
	e.gate <- struct{}{}
	defer e.unlock()
	out := Diagnostics{RuntimeVersion: e.runtimeVersion, Platform: runtime.GOOS + "/" + runtime.GOARCH, Backend: e.config.Backend, DeviceID: e.config.DeviceID, Threads: e.threads, Closed: e.closed, Stages: []StageDiagnostics{}}
	networks := []*network{e.detector, e.classifier}
	for _, r := range e.recognizers {
		networks = append(networks, r.model)
	}
	for _, n := range networks {
		if n == nil {
			continue
		}
		s := n.snapshotDiagnostics()
		out.Stages = append(out.Stages, s)
	}
	sort.Slice(out.Stages, func(i, j int) bool { return out.Stages[i].Stage < out.Stages[j].Stage })
	return out
}

func (n *network) recordRun(start time.Time) {
	n.diagnostics.Runs++
	n.diagnostics.RunSeconds += time.Since(start).Seconds()
}
