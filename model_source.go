package oneocr

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// A source owns one package descriptor or refers to a validated legacy directory.
// Package recognition reads one resource at a time, never the complete container.
type modelSource struct {
	bundle     *Bundle
	file       *os.File
	info       *PackageInfo
	entries    map[string]PackageFile
	dataOffset int64
}

func (s *modelSource) close() error {
	if s == nil || s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}
func (s *modelSource) reader(name string) (io.ReadCloser, FileInfo, error) {
	if !validRelative(name) {
		return nil, FileInfo{}, fmt.Errorf("invalid resource name %q", name)
	}
	if s.info != nil {
		entry, ok := s.entries[name]
		if !ok || s.file == nil {
			return nil, FileInfo{}, fmt.Errorf("package resource unavailable: %s", name)
		}
		return io.NopCloser(io.NewSectionReader(s.file, s.dataOffset+entry.Offset, entry.Bytes)), entry.FileInfo, nil
	}
	var info FileInfo
	if name == s.bundle.Config.File {
		info = s.bundle.Config
	} else {
		for _, r := range s.bundle.Resources {
			if r.File == name {
				info = r.FileInfo
				break
			}
		}
	}
	if info.File == "" {
		return nil, info, fmt.Errorf("unlisted bundle resource: %s", name)
	}
	f, err := os.Open(filepath.Join(s.bundle.directory, filepath.FromSlash(name)))
	if err != nil {
		return nil, info, err
	}
	return f, info, nil
}
func (s *modelSource) read(name string) ([]byte, error) {
	r, info, err := s.reader(name)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := readLimited(r, info.Bytes)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != info.Bytes || !strings.EqualFold(fmt.Sprintf("%x", sha256.Sum256(data)), info.SHA256) {
		return nil, fmt.Errorf("resource changed or checksum mismatch: %s", name)
	}
	return data, nil
}
func openSource(config Config) (*modelSource, error) {
	if (config.ModelPath == "") == (config.BundleDir == "") {
		return nil, fmt.Errorf("oneocr: specify exactly one of ModelPath or BundleDir")
	}
	if config.ModelPath != "" {
		return openPackage(config.ModelPath)
	}
	b, err := ReadBundle(config.BundleDir)
	if err != nil {
		return nil, err
	}
	return &modelSource{bundle: b}, nil
}
