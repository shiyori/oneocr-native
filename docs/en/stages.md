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

Coordinates refer to the EXIF-oriented input image with a zero origin. Full results contain `text`, `lines`, `width`, `height`, `elapsed_seconds`, `model_sha256` and `warnings`. Each line includes `text`, `quad`, `script`, `confidence` and `words`; the latter two remain null where the model pipeline does not provide calibrated output.

Detection results contain `regions`; each region has `quad`, `score` and `vertical`. Line results contain `text`, `script`, `rotated_180`, `elapsed_seconds` and `model_sha256`. Native C/C++/Android APIs return UTF-8 JSON; Go and Python provide typed results.

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)
