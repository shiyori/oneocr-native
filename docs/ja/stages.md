# 認識操作

[简体中文](../zh-CN/stages.md) | [English](../en/stages.md) | [日本語](../ja/stages.md)

SDK は三つの業務操作を提供します。

| 操作 | 入力 | 結果 |
|---|---|---|
| `Recognize` / `recognize` | ページまたは画像 | 文字列、行、四角形座標 |
| `Detect` / `detect` | ページまたは画像 | 検出領域、スコア、方向 |
| `RecognizeLine` / `recognize_line` | 切り出した横書きの一行 | 文字列、文字体系、180° 補正状態 |

各操作は言語ごとの同じ入力形式を受け取ります。File/Encoded/RGB ごとの操作メソッド群はありません。

`Recognize` は検出、文字体系の分類、行認識を実行します。`Detect` は分類と認識を省略し、検出段階のスコアを返します。`RecognizeLine` はページ検出を省略します。script を指定しない場合は分類と 180° 補正を行い、指定した場合は正方向の切り出し画像として扱います。

座標は EXIF の向きを適用した画像を基準とし、原点はゼロです。全体認識の結果は `text`、`lines`、`width`、`height`、`elapsed_seconds`、`model_sha256`、`warnings` を含みます。各行には `text`、`quad`、`script`、`confidence`、`words` があり、校正済み出力のないフィールドは null のままです。

検出結果の `regions` は `quad`、`score`、`vertical` を含みます。一行の結果は `text`、`script`、`rotated_180`、`elapsed_seconds`、`model_sha256` を含みます。C/C++/Android は UTF-8 JSON、Go と Python は型付きの結果を返します。

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)
