package engine

import (
	"archive/zip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed pipeline_spec.json
var pipelineSpec []byte

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("input exceeds %d byte limit", limit)
	}
	return b, nil
}
func validHash(s string) bool { b, e := hex.DecodeString(s); return e == nil && len(b) == sha256.Size }
func validRelative(s string) bool {
	return s != "" && s != "." && !strings.ContainsAny(s, "\\:\x00") && !path.IsAbs(s) && path.Clean(s) == s && s != ".." && !strings.HasPrefix(s, "../")
}
func (b *Bundle) Directory() string { return b.directory }
func (b *Bundle) Resolve(name string) (string, error) {
	if !validRelative(name) {
		return "", fmt.Errorf("non-portable bundle path %q", name)
	}
	for _, r := range b.Resources {
		if r.File == name {
			return filepath.Join(b.directory, filepath.FromSlash(name)), nil
		}
	}
	return "", fmt.Errorf("resource not listed in bundle: %s", name)
}
func pipelinePaths(p Pipeline) []string {
	out := []string{p.DetectorPath, p.ClassifierPath}
	for _, c := range p.Characters {
		for _, s := range []string{c.ModelPath, c.AlphabetPath, c.PhysicalMapPath, c.CompositePath, c.PriorPath} {
			if s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}
func validatePipeline(p Pipeline) error {
	if p.DetectorPath == "" || p.ClassifierPath == "" || len(p.Characters) == 0 || len(p.Characters) > 9 {
		return fmt.Errorf("missing or unsupported pipeline models")
	}
	seen := map[string]bool{}
	for _, c := range p.Characters {
		supported := false
		for _, s := range scripts {
			if c.Script == s {
				supported = true
			}
		}
		if !supported || seen[c.Script] || c.ModelPath == "" || c.AlphabetPath == "" || (c.PixelsPerFrame != 4 && c.PixelsPerFrame != 8) {
			return fmt.Errorf("invalid recognizer configuration for %q", c.Script)
		}
		seen[c.Script] = true
	}
	values := []float64{p.SegmentThreshold}
	for _, level := range []int{2, 3, 4} {
		v, ok := p.LineThresholds[level]
		if !ok {
			return fmt.Errorf("missing P%d threshold", level)
		}
		values = append(values, v)
	}
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 1 {
			return fmt.Errorf("invalid pipeline probability")
		}
	}
	return nil
}
func checkFile(root string, info FileInfo) error {
	if !validRelative(info.File) || !validHash(info.SHA256) || info.Bytes < 0 || info.Bytes > maxModelBytes {
		return fmt.Errorf("invalid bundle file metadata: %q", info.File)
	}
	filename := filepath.Join(root, filepath.FromSlash(info.File))
	stat, e := os.Lstat(filename)
	if e != nil {
		return e
	}
	if !stat.Mode().IsRegular() || stat.Size() != info.Bytes {
		return fmt.Errorf("bundle file size/type mismatch: %s", info.File)
	}
	resolved, e := filepath.EvalSymlinks(filename)
	if e != nil {
		return e
	}
	rel, e := filepath.Rel(root, resolved)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("bundle path escapes root")
	}
	f, e := os.Open(filename)
	if e != nil {
		return e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return e
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), info.SHA256) {
		return fmt.Errorf("bundle checksum mismatch: %s", info.File)
	}
	return nil
}

func validateBundleMetadata(b *Bundle) error {
	if b.Schema != BundleSchema || !validHash(b.SourceSHA256) {
		return fmt.Errorf("unsupported or invalid bundle schema")
	}
	if len(b.Resources) == 0 || len(b.Resources) > 4096 {
		return fmt.Errorf("invalid bundle resource count")
	}
	if e := validatePipeline(b.Pipeline); e != nil {
		return e
	}
	if !validRelative(b.Config.File) || !validHash(b.Config.SHA256) || b.Config.Bytes < 0 || b.Config.Bytes > maxModelBytes {
		return fmt.Errorf("invalid original configuration metadata")
	}
	files := map[string]bool{}
	ids := map[int]bool{}
	for _, r := range b.Resources {
		if files[r.File] || ids[r.ID] || r.ID < 0 || (r.Kind != "onnx" && r.Kind != "data") || !validRelative(r.File) || !validHash(r.SHA256) || r.Bytes < 0 || r.Bytes > maxModelBytes {
			return fmt.Errorf("duplicate or invalid bundle resource")
		}
		files[r.File] = true
		ids[r.ID] = true
	}
	for _, name := range pipelinePaths(b.Pipeline) {
		if !files[name] {
			return fmt.Errorf("missing pipeline resource: %s", name)
		}
	}
	return nil
}

// ReadBundle verifies every resource and required pipeline reference.
func ReadBundle(directory string) (*Bundle, error) {
	root, e := filepath.Abs(directory)
	if e != nil {
		return nil, e
	}
	root, e = filepath.EvalSymlinks(root)
	if e != nil {
		return nil, e
	}
	f, e := os.Open(filepath.Join(root, "bundle.json"))
	if e != nil {
		return nil, e
	}
	data, e := readLimited(f, 16*1024*1024)
	f.Close()
	if e != nil {
		return nil, e
	}
	var b Bundle
	if e = json.Unmarshal(data, &b); e != nil {
		return nil, e
	}
	if e = validateBundleMetadata(&b); e != nil {
		return nil, e
	}
	if e = checkFile(root, b.Config); e != nil {
		return nil, e
	}
	for _, r := range b.Resources {
		if e = checkFile(root, r.FileInfo); e != nil {
			return nil, e
		}
	}
	b.directory = root
	return &b, nil
}

// ExportModel exports all 67 resources of the validated model profile, without
// needing Python or ONNX Runtime. Resource counts come from the model itself.
// Existing valid bundles of the same source are reused; other targets are not overwritten.
func ExportModel(modelPath, destination string) (*Bundle, error) {
	m, e := readModel(modelPath)
	if e != nil {
		return nil, e
	}
	p, e := parsePipeline(m.config)
	if e != nil {
		return nil, e
	}
	target, e := filepath.Abs(destination)
	if e != nil {
		return nil, e
	}
	if _, e = os.Stat(target); e == nil {
		b, e := ReadBundle(target)
		if e == nil && b.SourceSHA256 == m.hash {
			return b, nil
		}
		return nil, fmt.Errorf("destination exists and is not this model's valid bundle: %s", target)
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
		return nil, e
	}
	stage, e := os.MkdirTemp(filepath.Dir(target), ".oneocr-export-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(stage)
	b := Bundle{Schema: BundleSchema, SourceSHA256: m.hash, Runtime: RuntimeInfo{"onnxruntime", "1.29.0", "CPUExecutionProvider", true}, Pipeline: p}
	names := map[string]string{}
	usedNames := map[string]bool{}
	for i, r := range m.resources {
		kind, filename := "data", resourceFilename(r.name)
		if !validRelative(filename) || usedNames[filename] {
			return nil, fmt.Errorf("invalid or colliding standard resource name: %s", filename)
		}
		usedNames[filename] = true
		var iface *ModelInterface
		if strings.HasSuffix(strings.ToLower(r.name), ".onnx") {
			kind = "onnx"
			iface, e = modelInterface(r.data)
			if e != nil {
				return nil, fmt.Errorf("ONNX resource %d: %w", i, e)
			}
		}
		name := filepath.Join(stage, filepath.FromSlash(filename))
		if e = os.MkdirAll(filepath.Dir(name), 0755); e != nil {
			return nil, e
		}
		if e = os.WriteFile(name, r.data, 0644); e != nil {
			return nil, e
		}
		names[r.name] = filename
		b.Resources = append(b.Resources, Resource{FileInfo: FileInfo{filename, int64(len(r.data)), fmt.Sprintf("%x", sha256.Sum256(r.data))}, ID: i, OriginalName: r.name, Kind: kind, Interface: iface})
	}
	rewrite := func(value *string) error {
		if *value == "" {
			return nil
		}
		name, ok := names[*value]
		if !ok {
			return fmt.Errorf("missing model dependency: %s", *value)
		}
		*value = name
		return nil
	}
	if e = rewrite(&b.Pipeline.DetectorPath); e != nil {
		return nil, e
	}
	if e = rewrite(&b.Pipeline.ClassifierPath); e != nil {
		return nil, e
	}
	for i := range b.Pipeline.Characters {
		c := &b.Pipeline.Characters[i]
		for _, s := range []*string{&c.ModelPath, &c.AlphabetPath, &c.PhysicalMapPath, &c.CompositePath, &c.PriorPath} {
			if e = rewrite(s); e != nil {
				return nil, e
			}
		}
	}
	b.Config = FileInfo{"config.pb", int64(len(m.config)), fmt.Sprintf("%x", sha256.Sum256(m.config))}
	if e = os.WriteFile(filepath.Join(stage, "config.pb"), m.config, 0644); e != nil {
		return nil, e
	}
	encoded, e := json.MarshalIndent(b, "", "  ")
	if e != nil {
		return nil, e
	}
	if e = os.WriteFile(filepath.Join(stage, "bundle.json"), append(encoded, '\n'), 0644); e != nil {
		return nil, e
	}
	if e = os.WriteFile(filepath.Join(stage, "pipeline.spec.json"), pipelineSpec, 0644); e != nil {
		return nil, e
	}
	if _, e = ReadBundle(stage); e != nil {
		return nil, e
	}
	if e = os.Rename(stage, target); e != nil {
		other, check := ReadBundle(target)
		if check == nil && other.SourceSHA256 == m.hash {
			return other, nil
		}
		return nil, e
	}
	return ReadBundle(target)
}

// ArchiveBundle packages an already verified bundle. Native runtime binaries
// are intentionally not included: each platform supplies its own ORT library.
func ArchiveBundle(directory, filename string) error {
	b, e := ReadBundle(directory)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if e != nil {
		return e
	}
	z := zip.NewWriter(f)
	ok := false
	defer func() {
		if !ok {
			z.Close()
			f.Close()
			os.Remove(filename)
		}
	}()
	names := []string{"bundle.json", "config.pb", "pipeline.spec.json"}
	for _, r := range b.Resources {
		names = append(names, r.File)
	}
	for _, name := range names {
		in, e := os.Open(filepath.Join(b.directory, filepath.FromSlash(name)))
		if e != nil {
			return e
		}
		out, e := z.Create(name)
		if e != nil {
			in.Close()
			return e
		}
		_, e = io.Copy(out, in)
		in.Close()
		if e != nil {
			return e
		}
	}
	if e = z.Close(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	ok = true
	return nil
}
