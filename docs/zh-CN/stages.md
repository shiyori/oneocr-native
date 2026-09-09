# 识别操作

[简体中文](../zh-CN/stages.md) | [English](../en/stages.md) | [日本語](../ja/stages.md)

SDK 保留三个业务操作：

| 操作 | 输入 | 结果 |
|---|---|---|
| `Recognize` / `recognize` | 页面或图片 | 文字、文本行和四边形坐标 |
| `Detect` / `detect` | 页面或图片 | 检测区域、分数和方向 |
| `RecognizeLine` / `recognize_line` | 已裁剪的单行水平文字 | 文字、书写系统和 180° 纠正状态 |

每种操作共用所在语言的输入类型，不再为 File/Encoded/RGB 分别创建一组方法。

`Recognize` 执行检测、书写系统分类和逐行识别。`Detect` 跳过分类和识别，返回的分数属于检测阶段。`RecognizeLine` 跳过页面检测；未指定 script 时会分类并纠正 180° 旋转，指定 script 时按正向裁剪图片处理。

坐标对应应用 EXIF 方向后的输入图像，原点为零。完整识别结果包括 `text`、`lines`、`width`、`height`、`elapsed_seconds`、`model_sha256`、`warnings`。每行包括 `text`、`quad`、`script`、`confidence`、`words`；未提供校准结果的字段保持 null。

检测结果的 `regions` 包含 `quad`、`score`、`vertical`。单行结果包含 `text`、`script`、`rotated_180`、`elapsed_seconds`、`model_sha256`。C/C++/Android 返回 UTF-8 JSON；Go 和 Python 返回有明确类型的结果。

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)
