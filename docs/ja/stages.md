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

`go install` で導入した CLI と Linux 完全パッケージは、完全な JSON 出力に対応します。オプションは画像パスの前に指定します。`recognize` の既定値はテキストで、`--format json` により構造化結果を出力します。`detect` は常に JSON です。標準出力をそのまま保存・解析でき、エラーは標準エラーと非ゼロ終了コードで通知します。

```sh
oneocr recognize --format json image.png > result.json
oneocr detect --format json image.png
oneocr recognize-line --format json line.png
```

Go の `Result` は JSON tag を持ち、`json.NewEncoder(writer).Encode(result)` で出力できます。Python は `result.to_dict()` と `json.dumps(..., ensure_ascii=False)` を使用します。C/C++/Android は同じフィールド構造の UTF-8 JSON を返します。

完全な認識結果の例です。スコアと処理時間は表示用に丸めています。

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

座標は EXIF 方向適用後の入力画像のピクセル単位で、原点は左上です。`coordinate_space="oriented_image"` と `width/height` はその画像を参照します。`quad` は領域の境界に沿った左上・右上・右下・左下の順で、`bbox` は軸に平行な外接矩形です。`vertical` は検出された縦書き領域、`rotated_180` は認識用クロップへの追加の 180° 補正を表します。

`confidence` は 0–1 の値で、計算方式は `ctc_token_geometric_mean` です。CTC の連続重複、blank、両端で除去された空白 token を除き、各 token の最初の出力フレームにおける全字表で正規化した確率の幾何平均を求めます。トップレベルの値は全返却行の token をまとめて集計し、行スコアの算術平均ではありません。文字フィルターによって残った字表を再正規化してスコアを上げることはありません。未校正のスコアであり、行全体が正しい確率を意味しません。空の結果は `confidence: null`、`text: ""`、`lines: []` になります。

`detection_score` は検出モデルの独立したスコアです。`words` は `null` のままで、行単位の枠を提供しますが単語・文字単位の座標アラインメントは未実装です。`warnings` には元の拒否・校正モデルを使用しないことなどの実行上の制限を保持します。

`Detect` は `coordinate_space`、`regions`、画像サイズ、処理時間、モデルハッシュを返し、各 region は `quad`、`bbox`、`score`、`vertical` を含みます。`RecognizeLine` は `text`、`confidence`、`confidence_method`、`script`、`rotated_180`、クロップ画像サイズ、処理時間、モデルハッシュを返します。検出を行わないため、行枠や検出スコアは含みません。

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)
