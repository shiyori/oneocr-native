# Go

[简体中文](../zh-CN/go.md) | [English](../en/go.md) | [日本語](../ja/go.md)

Download and extract a [complete desktop SDK](installation.md). Go 1.24+ and a C compiler are required. Put the following in your application's `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "github.com/shiyori/oneocr-native"
)

func main() {
    engine, err := oneocr.Open(oneocr.Config{})
    if err != nil { panic(err) }
    defer engine.Close()
    result, err := engine.Recognize(context.Background(), oneocr.FromFile("image.png"), oneocr.Options{})
    if err != nil { panic(err) }
    fmt.Println(result.Text)
}
```

Run these commands in **your application's directory**. Skip `go mod init` if it already has a `go.mod`.

```sh
go mod init example.com/ocr-app
/path/to/sdk/bin/oneocr install --go-project .
go run .
```

On Windows use the SDK's `bin\oneocr.exe`. The setup command copies the Go module into OneOCR's managed installation and configures your project; you do not need a repository checkout or a hand-written `replace` path. Add `--offline` to use only the dependencies included in the complete SDK. This does not change your application's ORT binding dependency.

With the core SDK, the same command prepares missing model/runtime resources first. The standalone [Go source package](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-go-sdk-0.1.0.zip) is also available for build systems that manage source dependencies themselves.

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
