# Go 接入

[简体中文](../zh-CN/go.md) | [English](../en/go.md) | [日本語](../ja/go.md)

需要 Go 1.24+ 和 C 编译器。在自己的项目中通过 Go 模块导入，不需要下载桌面 SDK：

```sh
go mod init example.com/ocr-app
go get github.com/shiyori/oneocr-native@v0.1.3
```

已有 `go.mod` 时跳过 `go mod init`。

将以下代码放入自己的 `main.go`。`Install` 用于准备缺失的模型与依赖；已经准备好的环境可直接 `Open`：

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

## 命令行接入

通过 Go 安装命令后，可在任意目录准备资源并识别图片：

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.3
oneocr install
oneocr recognize image.png
oneocr recognize --format json image.png
```

把 Go 的可执行文件目录（通常为 `GOPATH/bin`）加入 `PATH`。两种接入方式都自动查找、复用兼容 runtime；缺失依赖按需下载，普通调用无需配置 runtime 路径，也不会修改宿主的 ORT Go 绑定版本。

## 输入与操作

```go
file := oneocr.FromFile("image.png")
encoded := oneocr.FromEncoded(pngOrJPEG)
img := oneocr.FromImage(goImage)
pixels := oneocr.FromPixels(oneocr.Pixels{
    Data: rgba, Width: width, Height: height, Stride: stride,
    Format: oneocr.RGBA,
})
```

任一输入都可用于 `Recognize`、`Detect` 和 `RecognizeLine`。同步调用期间借用图片或字节，返回前不要修改。JPEG 编码输入会应用 EXIF 方向；透明像素合成到白底，像素缓冲区使用 sRGB 并支持带填充的行跨度。

```go
regions, err := engine.Detect(ctx, oneocr.FromFile("page.png"))
line, err := engine.RecognizeLine(ctx, oneocr.FromFile("line.png"), oneocr.Options{})
```

连续识别时复用 Engine。同一实例的操作会串行执行，Context 超时包含排队时间。`Close` 等待当前调用结束，可重复调用。`Warmup(ctx)` 预加载识别器；关闭后仍可读取 `Diagnostics()`。

可通过 `oneocr.Config` 调整 `Threads`、`MaxSide`、`CharacterClasses` 等选项。默认配置无需模型或运行库路径。另见[识别操作](stages.md)和[运行库复用](runtime.md)。

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)
