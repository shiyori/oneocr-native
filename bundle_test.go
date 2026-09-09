package oneocr

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectMalformedContainerAndProto(t *testing.T) {
	for _, data := range [][]byte{nil, {0}, {255, 255, 255, 255, 255, 255, 255, 255}, make([]byte, 32)} {
		if _, err := decodeModel(data); err == nil {
			t.Fatalf("accepted malformed container: %x", data)
		}
	}
	for _, data := range [][]byte{{0}, {0x0a, 0xff}, {0x0a, 4, 'x'}, {0x08, 0xff}, {0x0d, 1}, {0x0b}} {
		if _, err := parseProto(data); err == nil {
			t.Fatalf("accepted malformed protobuf: %x", data)
		}
	}
	for _, data := range [][]byte{nil, make([]byte, 15), make([]byte, 17), make([]byte, 48)} {
		if _, err := decryptRecord(data, modelKey, true); err == nil {
			t.Fatal("accepted invalid CBC record")
		}
	}
	if _, err := readLimited(bytes.NewReader(make([]byte, 10)), 9); err == nil {
		t.Fatal("ignored read limit")
	}
}
func TestBundlePathsAndIntegrity(t *testing.T) {
	for _, p := range []string{"", ".", "..", "../escape", "/absolute", "C:/file", "models\\file", "models/../file", "a//b", "a\x00b"} {
		if validRelative(p) {
			t.Errorf("accepted %q", p)
		}
	}
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "resource.bin")
	data := []byte("original resource")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	info := FileInfo{"resource.bin", int64(len(data)), fmt.Sprintf("%x", sha256.Sum256(data))}
	if err := checkFile(root, info); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("tampered resource"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkFile(root, info); err == nil {
		t.Fatal("accepted altered weights")
	}
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "resource.bin"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "outside")); err != nil {
		t.Skip("symlinks unavailable")
	}
	info.File = "outside/resource.bin"
	if err := checkFile(root, info); err == nil {
		t.Fatal("accepted parent symlink outside bundle")
	}
}
func TestZeroEngineAndInvalidDimensions(t *testing.T) {
	e := &Engine{}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Recognize(context.Background(), FromEncoded(nil), Options{}); err != ErrClosed {
		t.Fatalf("got %v", err)
	}
	if _, err := e.Recognize(context.Background(), FromPixels(Pixels{Data: []byte{1}, Width: int(^uint(0) >> 1), Height: 2, Stride: 6, Format: RGB}), Options{}); err == nil {
		t.Fatal("accepted overflow dimensions")
	}
}
