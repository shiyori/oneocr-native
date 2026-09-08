# Go SDK

Module: `github.com/shiyori/oneocr-native`. Requires Go ≥1.24, CGO and ONNX Runtime 1.29 CPU.

## Install

Place the default `oneocr-cjk-en.ocrpack` in `models/` under your working directory. The repository already has this layout.

```bash
go install ./cmd/oneocr
oneocr install --runtime /absolute/path/to/libonnxruntime.dylib
oneocr recognize image.png
oneocr recognize --format json image.png
```

Use `onnxruntime.dll` on Windows or `libonnxruntime.so` on Linux. The desktop SDK includes the runtime in `lib/`, so its `bin/oneocr install` requires no runtime argument. Installation copies the default model and runtime to the user configuration directory. `ONEOCR_HOME` or `--home` selects another installation directory.

For an application using a local checkout or the source SDK:

```bash
go mod init example.com/ocr-app
go mod edit -replace github.com/shiyori/oneocr-native=/absolute/path/to/oneocr-native
go get github.com/shiyori/oneocr-native
```

## Recognize a file

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    oneocr "github.com/shiyori/oneocr-native"
)

func main() {
    engine, err := oneocr.Open(oneocr.Config{})
    if err != nil { log.Fatal(err) }
    defer engine.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    result, err := engine.RecognizeFile(ctx, "image.png", oneocr.Options{})
    if err != nil { log.Fatal(err) }
    fmt.Println(result.Text)
    for _, line := range result.Lines {
        fmt.Println(line.Text, line.Quad)
    }
}
```

`Open(Config{})` finds the default filename in the working directory or its `models/`, beside the executable or its `models/`, in the parent SDK directory, then in an existing installation. It does not scan for other model files. `ONEOCR_MODEL` can set a deployment path. `ONEOCR_RUNTIME` or `Config.RuntimeLibrary` selects a runtime explicitly. `OpenInstalled("")` uses only the installed configuration.

## Image inputs

```go
// image.Image, such as an in-memory screenshot:
result, err := engine.Recognize(ctx, screenshot, oneocr.Options{})

// Packed RGB data: one row occupies stride bytes.
result, err = engine.RecognizeRGB(ctx, rgb, width, height, stride, oneocr.Options{})

// Separate text detection and cropped-line recognition:
regions, err := engine.DetectFile(ctx, "page.png")
line, err := engine.RecognizeLineFile(ctx, "line.png", oneocr.Options{})
```

Use [the stage API guide](STAGES.md) for the region and line result types. Full OCR results contain `Text`, line text and quadrilaterals, script names, image dimensions and elapsed time. Errors are returned separately.

## Configure the engine

```go
engine, err := oneocr.Open(oneocr.Config{
    Threads: 2,
    MaxSide: 1600,
})
```

`Threads` defaults to 2 and accepts 1–16. `MaxSide` defaults to 1600 and accepts 128–4096. Script detection is automatic when `Options{}` is used. Create an engine once, reuse it for the images you need, and close it when finished. Calls on an engine are synchronous and serialized; context cancellation includes time spent waiting for it. `Close` is idempotent.

## CLI

```bash
oneocr recognize --timeout 30s image.png
oneocr recognize --threads 2 --max-side 1600 --format json image.png
oneocr detect image.png
oneocr recognize-line line.png
```

Place flags before the image filename. `recognize` and `recognize-line` print text by default; `--format json` includes structured results. `detect` prints region JSON.

Source license: [AGPL-3.0-only](../LICENSE). Third-party models and dependencies retain their own rights. This unofficial project is shared for learning and discussion and provided without warranty.
