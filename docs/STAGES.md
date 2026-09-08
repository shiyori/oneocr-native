# 独立检测与单行识别

`Recognize` 保持完整图片 OCR。独立调用使用下列 API，均复用 Engine，并遵守同一调用串行、取消及 Close 规则：

| 输入 | 仅检测 | 单行识别 |
|---|---|---|
| Go `image.Image` | `Detect(ctx, img)` | `RecognizeLine(ctx, crop, options)` |
| PNG/JPEG/GIF 编码 | `DetectEncoded(ctx, data)` | `RecognizeLineEncoded(ctx, data, options)` |
| 文件路径 | `DetectFile(ctx, path)` | `RecognizeLineFile(ctx, path, options)` |
| RGB + stride | `DetectRGB(ctx, data, w, h, stride)` | `RecognizeLineRGB(ctx, data, w, h, stride, options)` |

检测返回 `DetectionResult.Regions`，每项含 `Quad`、`Score`、`Vertical`。坐标相对经过 EXIF 方向校正的原图，原点归零；保留浮点精度，区域顺序未定义。Score 是未校准的检测分数。检测不会执行分类器或文字识别器。

单行识别接受已经裁剪、校正为水平排列的文字行，不执行检测。`Options{}` 自动选择文字系统并纠正 180 度倒置；`Options{Script: "CJK"}` 等显式指定时跳过分类器，要求输入正向。它返回 `LineResult`（Text、Script、Rotated180、耗时、模型摘要），不生成检测框或校准置信度。自动分类为包外文字系统时返回错误；分类为非文字时返回空文本。

要组合独立步骤，调用方应按 Quad 做透视裁剪，并按 Vertical 校正方向，然后逐行调用识别。不要把整页直接交给单行识别；它不会自动拆行。现有 `Recognize` 已包含这些处理和阅读顺序整理。

```go
regions, err := engine.Detect(ctx, screenshot)
if err != nil { return err }
// lineCrop is a rectified horizontal crop produced by the caller.
line, err := engine.RecognizeLine(ctx, lineCrop, oneocr.Options{Script: "CJK"})
```

CLI：

```sh
oneocr detect page.png
oneocr recognize-line --script CJK --format json crop.png
```

`detect` 始终输出 JSON。两个命令共用线程数、字符类别和诊断参数；每次启动 CLI 都会重新打开 Engine。

C ABI 提供 `OneOCRDetectEncoded`、`OneOCRDetectRGB`、`OneOCRRecognizeLineEncoded`、`OneOCRRecognizeLineRGB`，均带 `timeout_ms`（0 表示无期限）。参数与结果所有权见 `sdk/include/oneocr.h`；返回的 JSON/error 由 `OneOCRFree` 释放。C++ 封装对应 `detect`、`detectRGB`、`recognizeLine`、`recognizeLineRGB` 方法，输入为缓冲区。

Python CPU API 提供 `engine.detect(image)` 与 `engine.recognize_line(crop, script="CJK")`，输入为路径或 PIL 图片，返回 dataclass，支持 `to_dict()`。Python CLI 同样提供 `detect`、`recognize-line`。

Android Java 封装提供 `detect(byte[])`、`recognizeLine(byte[])` 和 `recognizeLine(byte[], String script)`，返回 UTF-8 JSON。JNI 对接相同 C ABI；在后台线程调用。
