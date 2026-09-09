package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	oneocr "github.com/shiyori/oneocr-native"
)

// integrateGo configures a consumer using the module proxy shipped in Releases.
// Go's checksum/proxy settings are scoped to this subprocess only.
func integrateGo(project, home string, offline bool) error {
	root, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("Go project needs go.mod; run go mod init in your application directory first: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	sdkRoot := filepath.Dir(filepath.Dir(executable))
	if home == "" {
		home, err = oneocr.DefaultHome()
		if err != nil {
			return err
		}
	}
	module, err := copyGoModule(filepath.Join(sdkRoot, "go"), home)
	if err != nil {
		return err
	}
	proxy := filepath.Join(filepath.Dir(filepath.Dir(executable)), "go-proxy")
	if stat, err := os.Stat(proxy); err != nil || !stat.IsDir() {
		return fmt.Errorf("oneocr: Go module files are missing; use the complete or core desktop SDK")
	}
	proxyPath := filepath.ToSlash(proxy)
	if !strings.HasPrefix(proxyPath, "/") {
		proxyPath = "/" + proxyPath
	}
	source := (&url.URL{Scheme: "file", Path: proxyPath}).String()
	if !offline {
		source += ",https://proxy.golang.org"
	}
	environment := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOPROXY=") && !strings.HasPrefix(entry, "GOSUMDB=") {
			environment = append(environment, entry)
		}
	}
	edit := exec.Command("go", "mod", "edit", "-replace=github.com/shiyori/oneocr-native="+module)
	edit.Dir = root
	edit.Stdout = os.Stderr
	edit.Stderr = os.Stderr
	if err = edit.Run(); err != nil {
		return err
	}
	command := exec.Command("go", "get", "github.com/shiyori/oneocr-native@"+oneocr.ReleaseTag)
	command.Dir = root
	command.Env = append(environment, "GOPROXY="+source, "GOSUMDB=off")
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err = command.Run(); err != nil {
		return err
	}
	// Populate the included dependency graph so later offline Go commands do
	// not need the SDK's temporary proxy setting, even for graph inspection.
	arguments := []string{"mod", "download"}
	err = filepath.WalkDir(proxy, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".mod") {
			return nil
		}
		relative, err := filepath.Rel(proxy, path)
		if err != nil {
			return err
		}
		module, version, ok := strings.Cut(filepath.ToSlash(relative), "/@v/")
		if ok {
			arguments = append(arguments, module+"@"+strings.TrimSuffix(version, ".mod"))
		}
		return nil
	})
	if err != nil {
		return err
	}
	download := exec.Command("go", arguments...)
	download.Dir = root
	download.Env = command.Env
	download.Stdout = os.Stderr
	download.Stderr = os.Stderr
	return download.Run()
}

func copyGoModule(source, home string) (string, error) {
	var files []string
	hash := sha256.New()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("oneocr: unexpected symlink in Go SDK")
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		fmt.Fprintln(hash, filepath.ToSlash(relative))
		_, err = io.Copy(hash, input)
		input.Close()
		if err != nil {
			return err
		}
		files = append(files, relative)
		return nil
	})
	if err != nil {
		return "", err
	}
	if _, err = os.Stat(filepath.Join(source, "go.mod")); err != nil {
		return "", fmt.Errorf("oneocr: SDK Go sources missing")
	}
	target := filepath.Join(home, "sdk", oneocr.Version, hex.EncodeToString(hash.Sum(nil)), "go")
	if _, err = os.Stat(filepath.Join(target, "go.mod")); err == nil {
		return filepath.Abs(target)
	}
	parent := filepath.Dir(target)
	if err = os.MkdirAll(parent, 0755); err != nil {
		return "", err
	}
	temporary, err := os.MkdirTemp(parent, ".go-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temporary)
	for _, name := range files {
		destination := filepath.Join(temporary, name)
		if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return "", err
		}
		input, err := os.Open(filepath.Join(source, name))
		if err != nil {
			return "", err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			input.Close()
			return "", err
		}
		_, err = io.Copy(output, input)
		input.Close()
		closeErr := output.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	if err = os.Rename(temporary, target); err != nil {
		return "", err
	}
	return filepath.Abs(target)
}
