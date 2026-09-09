# Go 接入

[简体中文](../zh-CN/go.md) | [English](../en/go.md) | [日本語](../ja/go.md)

下载并解压[完整桌面 SDK](installation.md)。需要 Go 1.24+ 和 C 编译器。把以下代码放入自己应用的 `main.go`：

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

在**自己的应用目录**运行以下命令；已有 `go.mod` 时跳过 `go mod init`。

```sh
go mod init example.com/ocr-app
/path/to/sdk/bin/oneocr install --go-project .
go run .
```

Windows 使用 SDK 的 `bin\oneocr.exe`。准备命令会将 Go 模块保存到 OneOCR 管理的安装目录，并配置当前项目，无需克隆仓库或手写 `replace` 路径。加上 `--offline` 即可只使用完整包内的依赖；宿主的 ORT 绑定版本不会被修改。

精简包使用同样的命令，先补齐缺失的模型和运行库。自行管理源码依赖的构建系统也可下载独立的 [Go 源码包](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-go-sdk-0.1.0-rc.1.zip)。

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
