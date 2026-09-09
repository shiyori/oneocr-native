package oneocr

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Pinned official ONNX Runtime v1.29.0 archives. SHA256 and sizes are from
// github.com/microsoft/onnxruntime release metadata, not mutable latest URLs.
func upstreamRuntimeAsset(platform string) (releaseAsset, error) {
	switch platform {
	case "windows-amd64":
		return releaseAsset{Name: "onnxruntime-win-x64-1.29.0.zip", Kind: "runtime", Platform: platform,
			Bytes: 79645520, SHA256: "c9b4b7086b529ad814f428c1bad028e20a25d7dc0699836775faace4ab5b78b2"}, nil
	case "darwin-arm64":
		return releaseAsset{Name: "onnxruntime-osx-arm64-1.29.0.tgz", Kind: "runtime", Platform: platform,
			Bytes: 41578864, SHA256: "d0706fc34f315d8c88639d0a8c81f2e09e815f282cabed3493c06a054352cf92"}, nil
	default:
		return releaseAsset{}, fmt.Errorf("oneocr: unsupported upstream runtime platform %s", platform)
	}
}

func prepareUpstreamRuntime(source releaseSource, temporary, platform string) (string, error) {
	asset, err := upstreamRuntimeAsset(platform)
	if err != nil {
		return "", err
	}
	if ManagedRuntimeVersion != "1.29.0" {
		return "", fmt.Errorf("oneocr: upstream runtime catalog needs updating")
	}
	source.baseURL = "https://github.com/microsoft/onnxruntime/releases/download/v" + ManagedRuntimeVersion + "/"
	archive, err := source.download(asset, temporary)
	if err != nil {
		return "", err
	}
	directory := filepath.Join(temporary, "runtime")
	if err = os.Mkdir(directory, 0755); err != nil {
		return "", err
	}
	return extractUpstreamRuntime(archive, directory, platform)
}

// Extract an explicit allowlist; archive paths never become destination paths.
func extractUpstreamRuntime(archive, directory, platform string) (string, error) {
	asset, err := upstreamRuntimeAsset(platform)
	if err != nil {
		return "", err
	}
	root := strings.TrimSuffix(strings.TrimSuffix(asset.Name, ".zip"), ".tgz") + "/"
	library := "libonnxruntime.dylib"
	upstreamLibrary := "lib/libonnxruntime." + ManagedRuntimeVersion + ".dylib"
	if platform == "windows-amd64" {
		library, upstreamLibrary = "onnxruntime.dll", "lib/onnxruntime.dll"
	}
	allowed := map[string]string{root + upstreamLibrary: library,
		root + "LICENSE": "LICENSE", root + "ThirdPartyNotices.txt": "ThirdPartyNotices.txt"}
	if platform == "windows-amd64" {
		allowed[root+"lib/onnxruntime_providers_shared.dll"] = "onnxruntime_providers_shared.dll"
	}
	written := map[string]bool{}
	write := func(name string, size int64, regular bool, input io.Reader) error {
		name = strings.TrimPrefix(name, "./")
		destination, keep := allowed[name]
		if !keep {
			return nil
		}
		if !regular || size <= 0 || size > 256*1024*1024 || written[destination] {
			return fmt.Errorf("oneocr: invalid upstream runtime member %s", name)
		}
		path := filepath.Join(directory, destination)
		output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		count, copyErr := io.Copy(output, io.LimitReader(input, size+1))
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if count != size {
			return fmt.Errorf("oneocr: truncated upstream runtime member")
		}
		written[destination] = true
		return nil
	}
	if strings.HasSuffix(asset.Name, ".zip") {
		zipped, err := zip.OpenReader(archive)
		if err != nil {
			return "", err
		}
		defer zipped.Close()
		for _, entry := range zipped.File {
			if _, keep := allowed[strings.TrimPrefix(entry.Name, "./")]; !keep {
				continue
			}
			stream, err := entry.Open()
			if err != nil {
				return "", err
			}
			err = write(entry.Name, int64(entry.UncompressedSize64), entry.Mode().IsRegular(), stream)
			closeErr := stream.Close()
			if err != nil {
				return "", err
			}
			if closeErr != nil {
				return "", closeErr
			}
		}
	} else {
		file, err := os.Open(archive)
		if err != nil {
			return "", err
		}
		defer file.Close()
		compressed, err := gzip.NewReader(file)
		if err != nil {
			return "", err
		}
		defer compressed.Close()
		limited := &io.LimitedReader{R: compressed, N: 512*1024*1024 + 1}
		reader := tar.NewReader(limited)
		for {
			header, err := reader.Next()
			if err == io.EOF {
				if limited.N <= 0 {
					return "", fmt.Errorf("oneocr: upstream runtime archive exceeds extraction limit")
				}
				break
			}
			if err != nil {
				return "", err
			}
			if err = write(header.Name, header.Size, header.Typeflag == tar.TypeReg, reader); err != nil {
				return "", err
			}
		}
	}
	for _, name := range []string{library, "LICENSE", "ThirdPartyNotices.txt"} {
		if !written[name] {
			return "", fmt.Errorf("oneocr: upstream runtime is missing %s", name)
		}
	}
	record := runtimeManifest{Schema: "oneocr.runtime.v1", Platform: platform, Version: ManagedRuntimeVersion, Library: library}
	names := make([]string, 0, len(written))
	for name := range written {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(directory, name)
		stat, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		hash, err := fileHash(path)
		if err != nil {
			return "", err
		}
		record.Files = append(record.Files, FileInfo{File: name, Bytes: stat.Size(), SHA256: hash})
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(directory, "runtime.json"), append(data, '\n'), 0644); err != nil {
		return "", err
	}
	return filepath.Join(directory, library), nil
}
