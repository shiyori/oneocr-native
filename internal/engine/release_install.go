package engine

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const releaseURL = "https://github.com/shiyori/oneocr-native/releases/download/" + ReleaseTag + "/"

type releaseAsset struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Platform string `json:"platform,omitempty"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
}
type releaseManifest struct {
	Schema  string         `json:"schema"`
	Version string         `json:"version"`
	Assets  []releaseAsset `json:"assets"`
}
type runtimeManifest struct {
	Schema   string     `json:"schema"`
	Platform string     `json:"platform"`
	Version  string     `json:"version"`
	Library  string     `json:"library"`
	Files    []FileInfo `json:"files"`
}

// releaseSource reads only this SDK version's GitHub Release, or an explicitly
// selected offline directory. OCR paths never invoke it.
type releaseSource struct {
	directory string
	offline   bool
	baseURL   string // fixed upstream origin for pinned runtime archives only
}

func (s releaseSource) read(name string, limit int64) ([]byte, error) {
	stream, err := s.open(name)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	return readLimited(stream, limit)
}
func (s releaseSource) open(name string) (io.ReadCloser, error) {
	if filepath.Base(name) != name || strings.ContainsAny(name, "/\\") || name == "." || name == "" {
		return nil, fmt.Errorf("oneocr: invalid release asset name")
	}
	if s.directory != "" {
		return os.Open(filepath.Join(s.directory, name))
	}
	if s.offline {
		return nil, fmt.Errorf("oneocr: offline resources are missing; pass --source RESOURCE_DIRECTORY or prepare them online")
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	base := s.baseURL
	if base == "" {
		base = releaseURL
	}
	response, err := client.Get(base + name)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("oneocr: Release %s asset %s: HTTP %d", ReleaseTag, name, response.StatusCode)
	}
	return response.Body, nil
}
func (s releaseSource) manifest() (releaseManifest, error) {
	var manifest releaseManifest
	data, err := s.read("release-manifest.json", 4*1024*1024)
	if err != nil {
		return manifest, err
	}
	checks, err := s.read("SHA256SUMS", 4*1024*1024)
	if err != nil {
		return manifest, err
	}
	expected := ""
	for _, line := range strings.Split(string(checks), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == "release-manifest.json" {
			if expected != "" {
				return manifest, fmt.Errorf("oneocr: duplicate manifest checksum")
			}
			expected = fields[0]
		}
	}
	sum := sha256.Sum256(data)
	if !validHash(expected) || hex.EncodeToString(sum[:]) != expected {
		return manifest, fmt.Errorf("oneocr: release manifest checksum mismatch")
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if manifest.Schema != "oneocr.release.v1" || manifest.Version != Version {
		return manifest, fmt.Errorf("oneocr: release manifest version mismatch")
	}
	names := map[string]bool{}
	for _, asset := range manifest.Assets {
		if !validHash(asset.SHA256) || asset.Bytes <= 0 || asset.Bytes > 2*1024*1024*1024 || filepath.Base(asset.Name) != asset.Name || strings.ContainsAny(asset.Name, "/\\") || names[asset.Name] {
			return manifest, fmt.Errorf("oneocr: invalid/duplicate release asset")
		}
		names[asset.Name] = true
	}
	return manifest, nil
}
func (m releaseManifest) asset(kind, platform string) (releaseAsset, error) {
	var selected releaseAsset
	for _, asset := range m.Assets {
		if asset.Kind == kind && asset.Platform == platform {
			if selected.Name != "" {
				return selected, fmt.Errorf("oneocr: duplicate %s asset", kind)
			}
			selected = asset
		}
	}
	if selected.Name == "" {
		return selected, fmt.Errorf("oneocr: Release %s has no %s asset for %s", ReleaseTag, kind, platform)
	}
	return selected, nil
}
func (s releaseSource) download(asset releaseAsset, directory string) (string, error) {
	destination := filepath.Join(directory, asset.Name)
	if actual, err := fileHash(destination); err == nil && actual == asset.SHA256 {
		return destination, nil
	}
	input, err := s.open(asset.Name)
	if err != nil {
		return "", err
	}
	defer input.Close()
	output, err := os.CreateTemp(directory, ".download-")
	if err != nil {
		return "", err
	}
	defer output.Close()
	defer os.Remove(output.Name())
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(output, hash), io.LimitReader(input, asset.Bytes+1))
	if err != nil {
		return "", err
	}
	if count != asset.Bytes || hex.EncodeToString(hash.Sum(nil)) != asset.SHA256 {
		return "", fmt.Errorf("oneocr: checksum/length mismatch for %s", asset.Name)
	}
	if err = output.Sync(); err != nil {
		return "", err
	}
	if err = output.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(output.Name(), destination); err != nil {
		return "", err
	}
	return destination, nil
}
func unpackRuntime(archive, directory, platform string) (string, error) {
	if err := extractReleaseArchive(archive, directory); err != nil {
		return "", err
	}
	record, err := readRuntimeManifest(directory, platform)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, record.Library), nil
}
func extractReleaseArchive(archive, directory string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	var total uint64
	names := map[string]bool{}
	for _, file := range reader.File {
		name := file.Name
		if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || !filepath.IsLocal(filepath.FromSlash(name)) || file.Mode()&os.ModeSymlink != 0 || names[name] {
			return fmt.Errorf("oneocr: unsafe/duplicate runtime archive entry")
		}
		names[name] = true
		total += file.UncompressedSize64
		if file.UncompressedSize64 > 512*1024*1024 || total > 1024*1024*1024 {
			return fmt.Errorf("oneocr: runtime archive exceeds limits")
		}
		destination := filepath.Join(directory, filepath.FromSlash(name))
		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(destination, 0755); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		input, e := file.Open()
		if e != nil {
			return e
		}
		output, e := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if e != nil {
			input.Close()
			return e
		}
		_, e = io.Copy(output, io.LimitReader(input, 512*1024*1024+1))
		input.Close()
		closeErr := output.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
func readRuntimeManifest(directory, platform string) (runtimeManifest, error) {
	var record runtimeManifest
	manifest, err := os.Open(filepath.Join(directory, "runtime.json"))
	if err != nil {
		return record, err
	}
	data, err := readLimited(manifest, 1024*1024)
	manifest.Close()
	if err != nil {
		return record, err
	}
	if err = json.Unmarshal(data, &record); err != nil {
		return record, err
	}
	if record.Schema != "oneocr.runtime.v1" || record.Platform != platform || record.Version != ManagedRuntimeVersion || record.Library != runtimeLibraryName() {
		return record, fmt.Errorf("oneocr: runtime package platform/version mismatch")
	}
	found := false
	names := map[string]bool{}
	for _, file := range record.Files {
		if !filepath.IsLocal(filepath.FromSlash(file.File)) || strings.Contains(file.File, "\\") || names[file.File] || !validHash(file.SHA256) {
			return record, fmt.Errorf("oneocr: invalid runtime file record")
		}
		names[file.File] = true
		hash, err := fileHash(filepath.Join(directory, filepath.FromSlash(file.File)))
		if err != nil {
			return record, err
		}
		if hash != file.SHA256 {
			return record, fmt.Errorf("oneocr: runtime file checksum mismatch: %s", file.File)
		}
		if file.File == record.Library {
			found = true
		}
	}
	if !found {
		return record, fmt.Errorf("oneocr: runtime library missing from manifest")
	}
	return record, nil
}
func prepareInstallResources(options *InstallOptions, temporary string) error {
	if options.ModelPath == "" && options.Home != "" {
		if installed, err := LoadInstallation(options.Home); err == nil {
			options.ModelPath = installed.ModelPath
			if options.RuntimeLibrary == "" {
				options.RuntimeLibrary = installed.RuntimeLibrary
			}
		}
	}
	if options.ModelPath == "" {
		if defaults, err := DefaultConfig(); err == nil {
			options.ModelPath = defaults.ModelPath
		}
	}
	library, err := resolveRuntimeLibrary(options.RuntimeLibrary, options.ModelPath, options.BundleDir)
	if err != nil {
		return err
	}
	available := false
	if filepath.IsAbs(library) {
		if stat, e := os.Stat(library); e == nil && stat.Mode().IsRegular() {
			available = true
			options.RuntimeLibrary = library
		}
	}
	if options.ModelPath != "" && available {
		return nil
	}
	source := releaseSource{directory: options.SourceDirectory, offline: options.Offline}
	var manifest releaseManifest
	if options.ModelPath == "" || (!available && runtime.GOOS != "windows" && runtime.GOOS != "darwin") {
		manifest, err = source.manifest()
		if err != nil {
			return err
		}
	}
	if options.ModelPath == "" {
		asset, err := manifest.asset("model", "")
		if err != nil {
			return err
		}
		options.ModelPath, err = source.download(asset, temporary)
		if err != nil {
			return err
		}
	}
	if !available {
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			options.RuntimeLibrary, err = prepareUpstreamRuntime(source, temporary, runtime.GOOS+"-"+runtime.GOARCH)
			return err
		}
		asset, err := manifest.asset("runtime", runtime.GOOS+"-"+runtime.GOARCH)
		if err != nil {
			return err
		}
		archive, err := source.download(asset, temporary)
		if err != nil {
			return err
		}
		directory := filepath.Join(temporary, "runtime")
		if err = os.Mkdir(directory, 0755); err != nil {
			return err
		}
		options.RuntimeLibrary, err = unpackRuntime(archive, directory, runtime.GOOS+"-"+runtime.GOARCH)
		if err != nil {
			return err
		}
	}
	return nil
}
