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

通过 `go install` 安装的 CLI 和 Linux 完整包都支持完整 JSON；参数放在图片路径之前。`recognize` 默认输出纯文字，添加 `--format json` 输出结构化结果，`detect` 始终输出 JSON。标准输出可直接重定向或交给 JSON 解析器，错误写入标准错误并返回非零退出码。

```sh
oneocr recognize --format json image.png > result.json
oneocr detect --format json image.png
oneocr recognize-line --format json line.png
```

Go 返回有 JSON tag 的 `Result`，可使用 `json.NewEncoder(writer).Encode(result)`；Python 使用 `result.to_dict()`，再通过 `json.dumps(..., ensure_ascii=False)` 序列化。C/C++/Android 直接返回同一字段结构的 UTF-8 JSON。

以下为完整识别结果示例，分数和耗时作了显示舍入：

```json
{
  "text": "你好世界 日本語テスト 한국어 123",
  "confidence": 0.9678,
  "confidence_method": "ctc_token_geometric_mean",
  "coordinate_space": "oriented_image",
  "lines": [
    {
      "text": "你好世界 日本語テスト 한국어 123",
      "quad": [[27.394, 49.498], [661.396, 49.498], [661.396, 99.606], [27.394, 99.606]],
      "bbox": {"x": 27.394, "y": 49.498, "width": 634.002, "height": 50.108},
      "script": "CJK",
      "confidence": 0.9678,
      "detection_score": 0.9947,
      "vertical": false,
      "rotated_180": false,
      "rotation_degrees": 0,
      "words": null
    }
  ],
  "width": 1000,
  "height": 160,
  "elapsed_seconds": 0.1753,
  "model_sha256": "f6cef38b839012cd824abf8b854ee9fa4f87d4c5265440661d66b23f4fab5155",
  "warnings": ["Experimental final quad fitting, normalization and reading order; original rejection/calibration are not applied."]
}
```

坐标单位为像素，原点在应用 EXIF 方向后的输入图像左上角，`coordinate_space="oriented_image"`，`width/height` 也是该图像尺寸。`quad` 依次为文本区域的左上、右上、右下、左下四角（沿区域边界）；`bbox` 是这些点的轴对齐外接矩形。`vertical` 表示检测出的竖排区域，`rotated_180` 表示识别裁剪图是否又做了 180° 纠正。

`confidence` 范围为 0–1，计算方法是 `ctc_token_geometric_mean`：对 CTC 去重后的非空白输出 token，在首次输出帧取完整字表上的归一化概率，去除两端被裁掉的空白 token，再求几何平均。顶层置信度按所有返回行的 token 一起汇总，不能直接对行分数作算术平均。字符过滤不会重新归一化保留字表来抬高分数。它未经准确率校准，不能解读为“整行正确的概率”。空结果为 `confidence: null`、`text: ""`、`lines: []`。

`detection_score` 是检测模型的独立分数，不属于文字识别置信度。`words` 保留为 `null`，当前提供行级框，尚未实现词/字级坐标对齐。`warnings` 保留未启用原始拒识与校准等运行限制。

`Detect` 返回 `coordinate_space`、`regions`、图像尺寸、耗时和模型哈希；每个 region 包含 `quad`、`bbox`、`score`、`vertical`。`RecognizeLine` 返回 `text`、`confidence`、`confidence_method`、`script`、`rotated_180`、裁剪图尺寸、耗时和模型哈希；该操作不执行检测，因此没有行框或检测分数。

紧凑的短文本区域会参考同页可靠长文本的方向，避免把孤立数字误作竖排或将 `6/9` 误翻转。分类器未确定书写系统时，仅对检测分数达标、Latin/CJK 两个识别器读数一致且满足确认分数要求的短数字补识别。单独裁剪的短数字缺少页面方向依据时，弱翻转信号会保持输入方向。

`rotation_degrees` 表示识别前对矫正裁剪图实际执行的**顺时针**旋转角度，取值为 `0/90/180/270`。绘制识别文字时应使用其逆变换映射回 `quad`。`vertical` 保留检测方向信息，`rotated_180` 保留半周翻转兼容信息；短数字的实际旋转应以 `rotation_degrees` 为准。

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)
