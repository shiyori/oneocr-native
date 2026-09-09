# Recognition operations

[简体中文](../zh-CN/stages.md) | [English](../en/stages.md) | [日本語](../ja/stages.md)

The SDK exposes three business operations:

| Operation | Input | Result |
|---|---|---|
| `Recognize` / `recognize` | A page or image | Text, text lines and quadrilaterals |
| `Detect` / `detect` | A page or image | Detector regions, scores and orientation |
| `RecognizeLine` / `recognize_line` | One cropped horizontal line | Text, script and 180° correction |

Each operation accepts the same input forms for its language. There are no separate File/Encoded/RGB operation families.

`Recognize` runs detection, script classification and line recognition. `Detect` skips classification and recognition; its score is a detector score. `RecognizeLine` skips page detection. Without an explicit script it classifies the crop and corrects 180° rotation; with a script it assumes an upright crop.

The CLI installed with `go install` and the complete Linux package both support full JSON. Put flags before the image path. `recognize` defaults to text; use `--format json` for structured output. `detect` always emits JSON. Standard output can be redirected or parsed directly; errors go to standard error with a nonzero exit status.

```sh
oneocr recognize --format json image.png > result.json
oneocr detect --format json image.png
oneocr recognize-line --format json line.png
```

Go returns a `Result` with JSON tags: serialize it with `json.NewEncoder(writer).Encode(result)`. In Python, use `result.to_dict()` and `json.dumps(..., ensure_ascii=False)`. C/C++/Android return UTF-8 JSON with the same fields.

Example full recognition result, with scores and duration rounded for display:

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

Coordinates are pixels in the EXIF-oriented input image, with the origin at its top-left corner. `coordinate_space="oriented_image"` and `width/height` refer to that image. `quad` follows the region boundary in top-left, top-right, bottom-right, bottom-left order; `bbox` is its axis-aligned bounding rectangle. `vertical` identifies a detected vertical region; `rotated_180` records additional 180-degree correction of the recognition crop.

`confidence` is in 0–1, using `ctc_token_geometric_mean`: collapse consecutive CTC duplicates, omit blank tokens and trimmed boundary whitespace tokens, take each remaining token's normalized probability over the full alphabet at its first emission frame, then compute the geometric mean. The top-level value aggregates tokens across all returned lines, not an arithmetic average of line scores. Character filters do not renormalize the allowed alphabet to inflate confidence. This score is uncalibrated and is not the probability that an entire line is correct. Empty recognition returns `confidence: null`, `text: ""`, and `lines: []`.

`detection_score` is a separate detector score. `words` remains `null`: line boxes are provided, but word/character alignment is not implemented. `warnings` retains runtime limitations, including unused original rejection/calibration models.

`Detect` returns `coordinate_space`, `regions`, image dimensions, duration and model hash; each region contains `quad`, `bbox`, `score`, `vertical`. `RecognizeLine` returns `text`, `confidence`, `confidence_method`, `script`, `rotated_180`, crop dimensions, duration and model hash. It does not run detection and has no line boxes or detection scores.

Compact text regions use reliable longer text on the same page to resolve orientation, avoiding accidental vertical rotation of isolated digits and `6/9` flips. An undetermined script can trigger a guarded short-numeral fallback only when detector evidence and matching Latin/CJK recognition results satisfy the confirmation thresholds. Without page context, a cropped compact numeral preserves its input orientation when the flip evidence is weak.

`rotation_degrees` reports the actual **clockwise** rotation applied to the rectified recognition crop: `0/90/180/270`. Apply the inverse transform to place recognized text back in `quad`. `vertical` retains detector direction information and `rotated_180` retains half-turn compatibility information; use `rotation_degrees` for the actual correction of compact regions.

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)
