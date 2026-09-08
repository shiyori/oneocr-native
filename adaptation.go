package oneocr

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

const AdaptationSchema = "oneocr.adaptation.v1"

type AdaptedModel struct {
	FileInfo
	SourceSHA256 string   `json:"source_sha256"`
	Outputs      []string `json:"outputs"`
	Recipe       string   `json:"recipe"`
	Approximate  bool     `json:"approximate"`
}
type AdaptationManifest struct {
	Schema       string                  `json:"schema"`
	SourceSHA256 string                  `json:"source_sha256"`
	Backend      Backend                 `json:"backend"`
	Models       map[string]AdaptedModel `json:"models"`
}

func readAdaptation(directory string, b *Bundle) (*AdaptationManifest, error) {
	if directory == "" {
		return nil, nil
	}
	f, err := os.Open(filepath.Join(directory, "adaptation.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := readLimited(f, 4*1024*1024)
	if err != nil {
		return nil, err
	}
	var m AdaptationManifest
	if err = json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Schema != AdaptationSchema || m.SourceSHA256 != b.SourceSHA256 || !validBackend(m.Backend) || len(m.Models) == 0 {
		return nil, fmt.Errorf("oneocr: adaptation schema, source or backend mismatch")
	}
	resources := map[string]Resource{}
	for _, r := range b.Resources {
		resources[r.File] = r
	}
	for name, v := range m.Models {
		r, ok := resources[name]
		if !ok || r.Kind != "onnx" || v.SourceSHA256 != r.SHA256 || !validRelative(v.File) || !validHash(v.SHA256) || v.Bytes < 1 || v.Bytes > maxModelBytes || v.Recipe == "" {
			return nil, fmt.Errorf("oneocr: invalid adaptation entry %q", name)
		}
		if len(v.Outputs) == 0 {
			return nil, fmt.Errorf("oneocr: missing adaptation outputs: %s", name)
		}
	}
	return &m, nil
}
func readAdaptedModel(directory string, v AdaptedModel) ([]byte, error) {
	path := filepath.Join(directory, filepath.FromSlash(v.File))
	// Do not let a crafted manifest/symlink redirect model reads outside its root.
	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, err
	}
	actual, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root, actual)
	if err != nil || !validRelative(filepath.ToSlash(relative)) {
		return nil, fmt.Errorf("oneocr: adaptation escapes its directory")
	}
	f, err := os.Open(actual)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := readLimited(f, v.Bytes)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != v.Bytes || fmt.Sprintf("%x", sha256.Sum256(data)) != v.SHA256 {
		return nil, fmt.Errorf("oneocr: adaptation checksum mismatch: %s", v.File)
	}
	return data, nil
}
func (e *Engine) openNetwork(resource, stage string, inputs, outputs []string) (*network, error) {
	requested := e.config.stageBackend(stage)
	n := &network{inputs: append([]string(nil), inputs...), originalOutputs: append([]string(nil), outputs...), config: e.config, diagnostics: StageDiagnostics{Stage: stage, Requested: requested}}
	n.original = func() ([]byte, error) { return e.source.read(resource) }
	n.selected = n.original
	original, err := n.original()
	if err != nil {
		return nil, err
	}
	selected := original
	selectedOutputs := outputs
	if e.adaptation != nil && e.adaptation.Backend == requested {
		if v, ok := e.adaptation.Models[resource]; ok {
			n.selected = func() ([]byte, error) { return readAdaptedModel(e.config.AdaptationDir, v) }
			selected, err = readAdaptedModel(e.config.AdaptationDir, v)
			if err != nil {
				return nil, err
			}
			compact := reflect.DeepEqual(v.Outputs, []string{"token_ids", "invalid"})
			if !reflect.DeepEqual(v.Outputs, outputs) && !(compact && len(outputs) == 1 && outputs[0] == "logsoftmax") {
				return nil, fmt.Errorf("oneocr: incompatible adaptation output interface: %s", resource)
			}
			selectedOutputs = v.Outputs
			n.diagnostics.Recipe = v.Recipe
			n.diagnostics.Experimental = true
		}
	}
	if err = n.createSession(selected, requested, selectedOutputs); err != nil {
		if e.config.Fallback == FallbackError || (requested == BackendCPU && n.diagnostics.Recipe == "") {
			return nil, fmt.Errorf("oneocr %s %s: %w", stage, requested, err)
		}
		reason := err.Error()
		if err = n.createSession(original, BackendCPU, outputs); err != nil {
			return nil, fmt.Errorf("oneocr %s: %s; CPU fallback failed: %w", stage, reason, err)
		}
		n.fallbackUsed = true
		n.diagnostics.FallbackReason = reason
		n.diagnostics.Recipe = ""
		n.diagnostics.Experimental = false
	}
	return n, nil
}
