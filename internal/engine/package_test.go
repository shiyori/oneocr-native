package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func packageTestBundle(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	b := Bundle{Schema: BundleSchema, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("source"))), Runtime: RuntimeInfo{"onnxruntime", "1.29.0", "CPUExecutionProvider", true}}
	add := func(name, kind string) string {
		data := []byte("fixture " + name)
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
			t.Fatal(err)
		}
		b.Resources = append(b.Resources, Resource{FileInfo: FileInfo{name, int64(len(data)), fmt.Sprintf("%x", sha256.Sum256(data))}, ID: len(b.Resources), OriginalName: name, Kind: kind})
		return name
	}
	b.Pipeline = Pipeline{DetectorPath: add("models/detection/universal.onnx", "onnx"), ClassifierPath: add("models/classification/script_orientation.onnx", "onnx"), SegmentThreshold: 0.7, LineThresholds: map[int]float64{2: 0.7, 3: 0.8, 4: 0.8}}
	for _, script := range []string{"CJK", "Latin", "Cyrillic", "Arabic"} {
		model := add("models/recognition/"+script+".onnx", "onnx")
		alphabet := add("data/"+script+"/alphabet.txt", "data")
		b.Pipeline.Characters = append(b.Pipeline.Characters, CharacterModel{Name: script, Script: script, ModelPath: model, AlphabetPath: alphabet, PixelsPerFrame: 4})
	}
	config := []byte("original config remains provenance")
	b.Config = FileInfo{"config.pb", int64(len(config)), fmt.Sprintf("%x", sha256.Sum256(config))}
	if err := os.WriteFile(filepath.Join(root, "config.pb"), config, 0644); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "bundle.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	return root
}
func TestPackageRoundtripAndLifetime(t *testing.T) {
	source := packageTestBundle(t)
	filename := filepath.Join(t.TempDir(), "model.ocrpack")
	info, err := Pack(PackOptions{BundleDir: source, Output: filename})
	if err != nil {
		t.Fatal(err)
	}
	if info.Profile != "cjk-en" || len(info.Bundle.Pipeline.Characters) != 2 {
		t.Fatal("wrong default profile")
	}
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(PackOptions{BundleDir: source, Output: filename}); err == nil {
		t.Fatal("overwrote existing package")
	}
	if actual, _ := os.ReadFile(filename); !bytes.Equal(before, actual) {
		t.Fatal("changed destination")
	}
	if err = os.Chmod(filename, 0444); err != nil {
		t.Fatal(err)
	}
	s, err := openPackage(filename)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := s.file
	data, err := s.read(info.Bundle.Pipeline.DetectorPath)
	if err != nil || string(data) != "fixture models/detection/universal.onnx" {
		t.Fatal(string(data), err)
	}
	if err = s.close(); err != nil {
		t.Fatal(err)
	}
	if _, err = descriptor.ReadAt(make([]byte, 1), 0); !errors.Is(err, os.ErrClosed) {
		t.Fatal("descriptor not closed", err)
	}
	if _, err = s.read(info.Bundle.Pipeline.DetectorPath); err == nil {
		t.Fatal("read after close")
	}
	directory := filepath.Join(t.TempDir(), "unpacked")
	if _, err = Unpack(filename, directory); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(t.TempDir(), "repacked.ocrpack")
	if _, err = Pack(PackOptions{BundleDir: directory, Output: second}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(second)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("pack/unpack/repack changed bytes", err)
	}
	if _, err = Open(Config{ModelPath: filename, BundleDir: source}); err == nil {
		t.Fatal("accepted two model sources")
	}
}
func testRewriteIndex(t *testing.T, original []byte, change func(*PackageInfo)) []byte {
	t.Helper()
	indexLength := binary.LittleEndian.Uint64(original[16:24])
	dataOffset := binary.LittleEndian.Uint64(original[24:32])
	var info PackageInfo
	if err := json.Unmarshal(original[64:64+indexLength], &info); err != nil {
		t.Fatal(err)
	}
	change(&info)
	index, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	offset := aligned64(64 + int64(len(index)))
	out := make([]byte, int(offset)+len(original)-int(dataOffset))
	copy(out, original[:64])
	binary.LittleEndian.PutUint64(out[16:24], uint64(len(index)))
	binary.LittleEndian.PutUint64(out[24:32], uint64(offset))
	sum := sha256.Sum256(index)
	copy(out[32:64], sum[:])
	copy(out[64:], index)
	copy(out[offset:], original[dataOffset:])
	return out
}
func TestPackageRejectsMalformedContainers(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "valid.ocrpack")
	if _, err := Pack(PackOptions{BundleDir: packageTestBundle(t), Output: filename}); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func([]byte) []byte{
		"magic":           func(b []byte) []byte { b[0] ^= 1; return b },
		"version":         func(b []byte) []byte { binary.LittleEndian.PutUint32(b[8:12], 9); return b },
		"flags":           func(b []byte) []byte { b[12] = 1; return b },
		"index_bounds":    func(b []byte) []byte { binary.LittleEndian.PutUint64(b[16:24], ^uint64(0)); return b },
		"data_bounds":     func(b []byte) []byte { binary.LittleEndian.PutUint64(b[24:32], ^uint64(0)); return b },
		"index_digest":    func(b []byte) []byte { b[32] ^= 1; return b },
		"resource_digest": func(b []byte) []byte { b[len(b)-1] ^= 1; return b },
		"truncated":       func(b []byte) []byte { return b[:len(b)-1] },
		"trailing":        func(b []byte) []byte { return append(b, 1) },
		"duplicate": func(b []byte) []byte {
			return testRewriteIndex(t, b, func(i *PackageInfo) { i.Files[1].File = i.Files[0].File })
		},
		"overlap": func(b []byte) []byte { return testRewriteIndex(t, b, func(i *PackageInfo) { i.Files[1].Offset = 0 }) },
		"traversal": func(b []byte) []byte {
			return testRewriteIndex(t, b, func(i *PackageInfo) { i.Files[0].File = "../escape" })
		},
		"profile": func(b []byte) []byte { return testRewriteIndex(t, b, func(i *PackageInfo) { i.Profile = "unknown" }) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), name+".ocrpack")
			if err := os.WriteFile(p, mutate(append([]byte(nil), original...)), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadPackage(p); err == nil {
				t.Fatal("accepted malformed package")
			}
		})
	}
}
