package oneocr

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpstreamRuntimeAllowlist(t *testing.T) {
	for _, platform := range []string{"darwin-arm64", "windows-amd64"} {
		for _, malformed := range []string{"", "duplicate", "symlink", "missing-license"} {
			t.Run(platform+"/"+malformed, func(t *testing.T) {
				asset, err := upstreamRuntimeAsset(platform)
				if err != nil || !validHash(asset.SHA256) || !strings.Contains(asset.Name, ManagedRuntimeVersion) {
					t.Fatal(asset, err)
				}
				root := strings.TrimSuffix(strings.TrimSuffix(asset.Name, ".zip"), ".tgz") + "/"
				library := "lib/libonnxruntime." + ManagedRuntimeVersion + ".dylib"
				if platform == "windows-amd64" {
					library = "lib/onnxruntime.dll"
				}
				names := []string{"../../escape", "./" + root + library, "./" + root + "ThirdPartyNotices.txt"}
				if malformed != "missing-license" {
					names = append(names, "./"+root+"LICENSE")
				}
				if malformed == "duplicate" {
					names = append(names, root+library)
				}
				directory := t.TempDir()
				archive := filepath.Join(directory, asset.Name)
				file, err := os.Create(archive)
				if err != nil {
					t.Fatal(err)
				}
				if platform == "windows-amd64" {
					writer := zip.NewWriter(file)
					for _, name := range names {
						header := &zip.FileHeader{Name: name}
						if malformed == "symlink" && strings.HasSuffix(name, library) {
							header.SetMode(os.ModeSymlink | 0777)
						}
						entry, err := writer.CreateHeader(header)
						if err != nil {
							t.Fatal(err)
						}
						if _, err = entry.Write([]byte("data")); err != nil {
							t.Fatal(err)
						}
					}
					if err = writer.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					compressed := gzip.NewWriter(file)
					writer := tar.NewWriter(compressed)
					for _, name := range names {
						header := &tar.Header{Name: name, Size: 4, Mode: 0644, Typeflag: tar.TypeReg}
						if malformed == "symlink" && strings.HasSuffix(name, library) {
							header.Typeflag = tar.TypeSymlink
							header.Size = 0
							header.Linkname = "outside"
						}
						if err = writer.WriteHeader(header); err != nil {
							t.Fatal(err)
						}
						if header.Size > 0 {
							if _, err = writer.Write([]byte("data")); err != nil {
								t.Fatal(err)
							}
						}
					}
					if err = writer.Close(); err != nil {
						t.Fatal(err)
					}
					if err = compressed.Close(); err != nil {
						t.Fatal(err)
					}
				}
				if err = file.Close(); err != nil {
					t.Fatal(err)
				}
				output := filepath.Join(directory, "runtime")
				if err = os.Mkdir(output, 0755); err != nil {
					t.Fatal(err)
				}
				path, err := extractUpstreamRuntime(archive, output, platform)
				if malformed != "" {
					if err == nil {
						t.Fatal("accepted malformed archive")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if data, err := os.ReadFile(path); err != nil || string(data) != "data" {
					t.Fatal("wrong library", err)
				}
				if _, err = os.Stat(filepath.Join(directory, "escape")); !os.IsNotExist(err) {
					t.Fatal("extracted an unlisted path")
				}
				if _, err = os.Stat(filepath.Join(output, "runtime.json")); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
