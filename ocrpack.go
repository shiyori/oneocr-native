package oneocr

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const PackageSchema = "oneocr.pack.v1"
const packageMagic = "ONEOCRPK"
const packageHeaderSize int64 = 64
const maxIndexBytes int64 = 16 * 1024 * 1024

func aligned64(n int64) int64 { return (n + 63) &^ 63 }

func validatePackageIndex(info *PackageInfo, dataSize int64) (map[string]PackageFile, error) {
	if info.Schema != PackageSchema {
		return nil, fmt.Errorf("unsupported package index schema")
	}
	allowed, err := profileScripts(info.Profile)
	if err != nil {
		return nil, err
	}
	b := &info.Bundle
	if err = validateBundleMetadata(b); err != nil {
		return nil, err
	}
	if len(b.Pipeline.Characters) != len(allowed) {
		return nil, fmt.Errorf("package profile and recognizers disagree")
	}
	for _, c := range b.Pipeline.Characters {
		if !allowed[c.Script] {
			return nil, fmt.Errorf("script %s is outside package profile", c.Script)
		}
	}
	if len(info.Files) != len(b.Resources)+2 {
		return nil, fmt.Errorf("package requires resources, original config and pipeline specification")
	}
	expected := map[string]FileInfo{b.Config.File: b.Config}
	for _, r := range b.Resources {
		if _, exists := expected[r.File]; exists {
			return nil, fmt.Errorf("package config/resource name collision")
		}
		expected[r.File] = r.FileInfo
	}
	if _, exists := expected["pipeline.spec.json"]; exists {
		return nil, fmt.Errorf("reserved pipeline specification name")
	}
	entries := make(map[string]PackageFile, len(info.Files))
	seenFold := map[string]bool{}
	previous := ""
	var end int64
	for _, f := range info.Files {
		if !validRelative(f.File) || !validHash(f.SHA256) || f.Bytes < 0 || f.Bytes > maxModelBytes || f.Offset < 0 || f.Offset%64 != 0 || f.Offset != aligned64(end) || f.Offset > dataSize || f.Bytes > dataSize-f.Offset {
			return nil, fmt.Errorf("invalid package resource extent: %s", f.File)
		}
		if f.File <= previous || seenFold[strings.ToLower(f.File)] {
			return nil, fmt.Errorf("package resource names must be unique and sorted")
		}
		if want, ok := expected[f.File]; ok {
			if want.Bytes != f.Bytes || !strings.EqualFold(want.SHA256, f.SHA256) {
				return nil, fmt.Errorf("inconsistent package metadata: %s", f.File)
			}
			delete(expected, f.File)
		} else if f.File != "pipeline.spec.json" {
			return nil, fmt.Errorf("unlisted package entry: %s", f.File)
		}
		entries[f.File] = f
		seenFold[strings.ToLower(f.File)] = true
		previous = f.File
		end = f.Offset + f.Bytes
	}
	if len(expected) != 0 || end != dataSize {
		return nil, fmt.Errorf("package index does not cover declared files or data section")
	}
	if _, ok := entries["pipeline.spec.json"]; !ok {
		return nil, fmt.Errorf("missing pipeline specification")
	}
	// Runtime packages include exactly the profile dependency closure.
	needed := map[string]bool{}
	for _, p := range pipelinePaths(b.Pipeline) {
		needed[p] = true
	}
	if len(needed) != len(b.Resources) {
		return nil, fmt.Errorf("package has unused or missing profile resources")
	}
	return entries, nil
}

func openPackage(filename string) (_ *modelSource, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			f.Close()
		}
	}()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() < packageHeaderSize || stat.Size() > maxModelBytes {
		return nil, fmt.Errorf("invalid package file type or size")
	}
	header := make([]byte, packageHeaderSize)
	if _, err = f.ReadAt(header, 0); err != nil {
		return nil, err
	}
	if string(header[:8]) != packageMagic {
		return nil, fmt.Errorf("not an ONEOCRPK container; use oneocr pack to convert OneModel/ZIP resources")
	}
	if binary.LittleEndian.Uint32(header[8:12]) != 1 || binary.LittleEndian.Uint32(header[12:16]) != 0 {
		return nil, fmt.Errorf("unsupported package version or flags")
	}
	indexSize := binary.LittleEndian.Uint64(header[16:24])
	dataOffset := binary.LittleEndian.Uint64(header[24:32])
	if indexSize == 0 || indexSize > uint64(maxIndexBytes) || dataOffset != uint64(aligned64(packageHeaderSize+int64(indexSize))) || dataOffset > uint64(stat.Size()) {
		return nil, fmt.Errorf("invalid package index bounds")
	}
	index := make([]byte, int(indexSize))
	if _, err = f.ReadAt(index, packageHeaderSize); err != nil {
		return nil, err
	}
	digest := sha256.Sum256(index)
	if !bytes.Equal(digest[:], header[32:64]) {
		return nil, fmt.Errorf("package index checksum mismatch")
	}
	var info PackageInfo
	if err = json.Unmarshal(index, &info); err != nil {
		return nil, fmt.Errorf("invalid package index JSON: %w", err)
	}
	entries, err := validatePackageIndex(&info, stat.Size()-int64(dataOffset))
	if err != nil {
		return nil, err
	}
	// Stream checksums with a bounded buffer before passing any model to ORT.
	buffer := make([]byte, 64*1024)
	for _, entry := range info.Files {
		h := sha256.New()
		if _, err = io.CopyBuffer(h, io.NewSectionReader(f, int64(dataOffset)+entry.Offset, entry.Bytes), buffer); err != nil {
			return nil, err
		}
		if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), entry.SHA256) {
			return nil, fmt.Errorf("package resource checksum mismatch: %s", entry.File)
		}
	}
	s := &modelSource{bundle: &info.Bundle, file: f, info: &info, entries: entries, dataOffset: int64(dataOffset)}
	success = true
	return s, nil
}

// ReadPackage verifies the complete container without creating an ORT session.
func ReadPackage(filename string) (*PackageInfo, error) {
	s, err := openPackage(filename)
	if err != nil {
		return nil, err
	}
	defer s.close()
	return s.info, nil
}
func profileScripts(profile string) (map[string]bool, error) {
	switch profile {
	case "cjk-en":
		return map[string]bool{"CJK": true, "Latin": true}, nil
	case "extended":
		return map[string]bool{"CJK": true, "Latin": true, "Cyrillic": true, "Arabic": true}, nil
	default:
		return nil, fmt.Errorf("unknown profile %q; use cjk-en or extended", profile)
	}
}
func selectProfile(b *Bundle, profile string) (Bundle, error) {
	allowed, err := profileScripts(profile)
	if err != nil {
		return Bundle{}, err
	}
	out := *b
	out.Pipeline = b.Pipeline
	out.Pipeline.Characters = nil
	out.Resources = nil
	for _, c := range b.Pipeline.Characters {
		if allowed[c.Script] {
			out.Pipeline.Characters = append(out.Pipeline.Characters, c)
		}
	}
	if len(out.Pipeline.Characters) != len(allowed) {
		return Bundle{}, fmt.Errorf("source lacks one or more %s recognizers", profile)
	}
	needed := map[string]bool{}
	for _, name := range pipelinePaths(out.Pipeline) {
		needed[name] = true
	}
	for _, r := range b.Resources {
		if needed[r.File] {
			out.Resources = append(out.Resources, r)
		}
	}
	sort.Slice(out.Resources, func(i, j int) bool { return out.Resources[i].File < out.Resources[j].File })
	return out, validateBundleMetadata(&out)
}

// Pack writes a new custom, uncompressed .ocrpack. ModelPath is an original
// .onemodel; BundleDir is a standard directory, including a developer-edited one.
// Conversion may use temporary files; recognition never extracts a package.
func Pack(options PackOptions) (*PackageInfo, error) {
	if (options.ModelPath == "") == (options.BundleDir == "") || options.Output == "" {
		return nil, fmt.Errorf("pack requires one source and Output")
	}
	if options.Profile == "" {
		options.Profile = "cjk-en"
	}
	if _, err := profileScripts(options.Profile); err != nil {
		return nil, err
	}
	directory := options.BundleDir
	if options.ModelPath != "" {
		temporary, err := os.MkdirTemp("", "oneocr-pack-source-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(temporary)
		directory = filepath.Join(temporary, "bundle")
		if _, err = ExportModel(options.ModelPath, directory); err != nil {
			return nil, err
		}
	}
	source, err := openSource(Config{BundleDir: directory})
	if err != nil {
		return nil, err
	}
	defer source.close()
	b, err := selectProfile(source.bundle, options.Profile)
	if err != nil {
		return nil, err
	}
	spec := pipelineSpec
	specPath := filepath.Join(source.bundle.directory, "pipeline.spec.json")
	if stat, e := os.Lstat(specPath); e == nil {
		if !stat.Mode().IsRegular() {
			return nil, fmt.Errorf("pipeline specification must be a regular file")
		}
		f, e := os.Open(specPath)
		if e != nil {
			return nil, e
		}
		spec, e = readLimited(f, maxIndexBytes)
		f.Close()
		if e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if !json.Valid(spec) {
		return nil, fmt.Errorf("invalid pipeline specification JSON")
	}
	info := PackageInfo{Schema: PackageSchema, Profile: options.Profile, Bundle: b}
	infos := []FileInfo{b.Config, {File: "pipeline.spec.json", Bytes: int64(len(spec)), SHA256: fmt.Sprintf("%x", sha256.Sum256(spec))}}
	for _, r := range b.Resources {
		infos = append(infos, r.FileInfo)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].File < infos[j].File })
	var dataSize int64
	for _, entry := range infos {
		offset := aligned64(dataSize)
		info.Files = append(info.Files, PackageFile{FileInfo: entry, Offset: offset})
		dataSize = offset + entry.Bytes
	}
	if _, err = validatePackageIndex(&info, dataSize); err != nil {
		return nil, err
	}
	index, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	if int64(len(index)) > maxIndexBytes {
		return nil, fmt.Errorf("package index is too large")
	}
	dataOffset := aligned64(packageHeaderSize + int64(len(index)))
	if dataOffset+dataSize > maxModelBytes {
		return nil, fmt.Errorf("package exceeds 2 GiB limit")
	}
	target, err := filepath.Abs(options.Output)
	if err != nil {
		return nil, err
	}
	if _, err = os.Lstat(target); err == nil {
		return nil, fmt.Errorf("package destination already exists: %s", target)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".ocrpack-")
	if err != nil {
		return nil, err
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	defer f.Close()
	header := make([]byte, packageHeaderSize)
	copy(header, packageMagic)
	binary.LittleEndian.PutUint32(header[8:12], 1)
	binary.LittleEndian.PutUint64(header[16:24], uint64(len(index)))
	binary.LittleEndian.PutUint64(header[24:32], uint64(dataOffset))
	digest := sha256.Sum256(index)
	copy(header[32:64], digest[:])
	if _, err = f.Write(header); err != nil {
		return nil, err
	}
	if _, err = f.Write(index); err != nil {
		return nil, err
	}
	if err = f.Truncate(dataOffset + dataSize); err != nil {
		return nil, err
	}
	for _, entry := range info.Files {
		if _, err = f.Seek(dataOffset+entry.Offset, io.SeekStart); err != nil {
			return nil, err
		}
		var r io.ReadCloser
		if entry.File == "pipeline.spec.json" {
			r = io.NopCloser(bytes.NewReader(spec))
		} else {
			r, _, err = source.reader(entry.File)
			if err != nil {
				return nil, err
			}
		}
		h := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(r, entry.Bytes+1))
		r.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if n != entry.Bytes || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), entry.SHA256) {
			return nil, fmt.Errorf("source resource changed while packing: %s", entry.File)
		}
	}
	if err = f.Chmod(0644); err != nil {
		return nil, err
	}
	if err = f.Sync(); err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	if _, err = ReadPackage(temporary); err != nil {
		return nil, err
	}
	// Link publishes atomically without overwriting an existing developer file.
	if err = os.Link(temporary, target); err != nil {
		return nil, fmt.Errorf("publish package: %w", err)
	}
	return &info, nil
}

// Unpack restores a new standard bundle directory for developer customization.
func Unpack(filename, directory string) (*Bundle, error) {
	source, err := openPackage(filename)
	if err != nil {
		return nil, err
	}
	defer source.close()
	target, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if _, err = os.Lstat(target); err == nil {
		return nil, fmt.Errorf("unpack destination already exists: %s", target)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".ocrpack-unpack-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	for _, entry := range source.info.Files {
		dest := filepath.Join(stage, filepath.FromSlash(entry.File))
		if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return nil, err
		}
		data, err := source.read(entry.File)
		if err != nil {
			return nil, err
		}
		if err = os.WriteFile(dest, data, 0644); err != nil {
			return nil, err
		}
	}
	data, err := json.MarshalIndent(source.bundle, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(stage, "bundle.json"), append(data, '\n'), 0644); err != nil {
		return nil, err
	}
	if _, err = ReadBundle(stage); err != nil {
		return nil, err
	}
	if err = os.Rename(stage, target); err != nil {
		return nil, err
	}
	return ReadBundle(target)
}
