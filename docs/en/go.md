# Go

[简体中文](../zh-CN/go.md) | [English](../en/go.md) | [日本語](../ja/go.md)

Requires Go 1.24+ and a C compiler. Add the Go module to your own project; no desktop SDK download is needed:

```sh
go mod init example.com/ocr-app
go get github.com/shiyori/oneocr-native@v0.1.3
```

Skip `go mod init` if the project already has `go.mod`.

Place this in your own `main.go`. `Install` prepares missing models and dependencies; an already prepared environment can call `Open` directly:

```go
package main

import (
    "context"
    "encoding/json"
    "os"
    "github.com/shiyori/oneocr-native"
)

func main() {
    if _, err := oneocr.Install(oneocr.InstallOptions{}); err != nil { panic(err) }
    engine, err := oneocr.Open(oneocr.Config{})
    if err != nil { panic(err) }
    defer engine.Close()
    result, err := engine.Recognize(context.Background(), oneocr.FromFile("image.png"), oneocr.Options{})
    if err != nil { panic(err) }
    if err := json.NewEncoder(os.Stdout).Encode(result); err != nil { panic(err) }
}
```

```sh
go run .
```

## Command-line installation

Install the command with Go, then prepare resources and recognize images from any directory:

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.3
oneocr install
oneocr recognize image.png
oneocr recognize --format json image.png
```

Add the Go executable directory (usually `GOPATH/bin`) to `PATH`. Both paths discover and reuse a compatible runtime, downloading missing dependencies on demand. Normal calls need no runtime path and do not change the host’s ORT Go binding version.

## Inputs and operations

```go
file := oneocr.FromFile("image.png")
encoded := oneocr.FromEncoded(pngOrJPEG)
img := oneocr.FromImage(goImage)
pixels := oneocr.FromPixels(oneocr.Pixels{
    Data: rgba, Width: width, Height: height, Stride: stride,
    Format: oneocr.RGBA,
})
```

Use any input with `Recognize`, `Detect` or `RecognizeLine`. Inputs borrow bytes/images for the synchronous call; do not mutate them until it returns. Encoded JPEG inputs apply EXIF orientation. Transparent pixels are composited on white; pixel buffers use sRGB and may have padded row strides.

```go
regions, err := engine.Detect(ctx, oneocr.FromFile("page.png"))
line, err := engine.RecognizeLine(ctx, oneocr.FromFile("line.png"), oneocr.Options{})
```

Reuse an engine for repeated work. Operations on one engine serialize; a context deadline includes queue time. `Close` waits for active work and is idempotent. `Warmup(ctx)` loads all recognizers; `Diagnostics()` remains available after close.

Optional configuration belongs in `oneocr.Config`, including `Threads`, `MaxSide` and `CharacterClasses`. Default values work without model or runtime paths. See [operations](stages.md) and [runtime reuse](runtime.md).

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)
