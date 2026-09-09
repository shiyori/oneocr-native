# PaddleOCR 元画像のテスト結果

[简体中文](../zh-CN/test-results.md) | [English](../en/test-results.md) | [日本語](../ja/test-results.md)

`testdata/paddleocr` にある全 9 枚の元画像を、公開版 OneOCR v0.1.3 と既定の中国語・日本語・韓国語・英語モデルで再認識しました。入力は元の解像度を維持し、720p には変換していません。

比較画像には認識信頼度 ≥ **0.70**、検出スコア ≥ **0.70** の空でない結果のみを表示し、JSON には全認識結果を保持します。信頼度は未校正のモデルスコアです。完全な手動正解データはないため、以下の件数は正解率ではありません。

左は元画像に枠を追加し、右は同じ位置に青い認識文字と局所的な明るい背景を重ねています。両方とも全画面を保持して 2 倍で描画し、JSON の座標は元画像に対応します。文字の手動修正はありません。画像をクリックすると拡大できます。

| 元画像 | 元の解像度 | 空でない結果 | 表示結果 | 生の JSON |
|---|---|---:|---:|---|
| [book.jpg](#sample-book) | 1100 × 708 | 44 | 43 | [JSON](../assets/paddleocr-results/book.json) |
| [book_rot180.jpg](#sample-book_rot180) | 1100 × 708 | 46 | 46 | [JSON](../assets/paddleocr-results/book_rot180.json) |
| [doc_with_formula.png](#sample-doc_with_formula) | 816 × 1056 | 116 | 102 | [JSON](../assets/paddleocr-results/doc_with_formula.json) |
| [formula.png](#sample-formula) | 448 × 64 | 6 | 5 | [JSON](../assets/paddleocr-results/formula.json) |
| [medal_table.png](#sample-medal_table) | 550 × 345 | 96 | 96 | [JSON](../assets/paddleocr-results/medal_table.json) |
| [seal.png](#sample-seal) | 640 × 640 | 10 | 6 | [JSON](../assets/paddleocr-results/seal.json) |
| [table.jpg](#sample-table) | 551 × 132 | 12 | 12 | [JSON](../assets/paddleocr-results/table.json) |
| [textline.png](#sample-textline) | 514 × 64 | 1 | 1 | [JSON](../assets/paddleocr-results/textline.json) |
| [textline_rot180.jpg](#sample-textline_rot180) | 429 × 48 | 1 | 1 | [JSON](../assets/paddleocr-results/textline_rot180.json) |

<a id="sample-book"></a>

## book.jpg

元の解像度：1100 × 708；空でない結果 **43 / 44** 件を表示。

[元画像](../../testdata/paddleocr/book.jpg) · [JSON](../assets/paddleocr-results/book.json)

| 元画像 + テキスト枠 | 元画像 + 青い認識文字 |
|---|---|
| [![book.jpg — 元画像 + テキスト枠](../assets/paddleocr-results/book-boxes.webp)](../assets/paddleocr-results/book-boxes.webp) | [![book.jpg — 元画像 + 青い認識文字](../assets/paddleocr-results/book-text.webp)](../assets/paddleocr-results/book-text.webp) |

<a id="sample-book_rot180"></a>

## book_rot180.jpg

元の解像度：1100 × 708；空でない結果 **46 / 46** 件を表示。

[元画像](../../testdata/paddleocr/book_rot180.jpg) · [JSON](../assets/paddleocr-results/book_rot180.json)

| 元画像 + テキスト枠 | 元画像 + 青い認識文字 |
|---|---|
| [![book_rot180.jpg — 元画像 + テキスト枠](../assets/paddleocr-results/book_rot180-boxes.webp)](../assets/paddleocr-results/book_rot180-boxes.webp) | [![book_rot180.jpg — 元画像 + 青い認識文字](../assets/paddleocr-results/book_rot180-text.webp)](../assets/paddleocr-results/book_rot180-text.webp) |

<a id="sample-doc_with_formula"></a>

## doc_with_formula.png

元の解像度：816 × 1056；空でない結果 **102 / 116** 件を表示。

[元画像](../../testdata/paddleocr/doc_with_formula.png) · [JSON](../assets/paddleocr-results/doc_with_formula.json)

| 元画像 + テキスト枠 | 元画像 + 青い認識文字 |
|---|---|
| [![doc_with_formula.png — 元画像 + テキスト枠](../assets/paddleocr-results/doc_with_formula-boxes.webp)](../assets/paddleocr-results/doc_with_formula-boxes.webp) | [![doc_with_formula.png — 元画像 + 青い認識文字](../assets/paddleocr-results/doc_with_formula-text.webp)](../assets/paddleocr-results/doc_with_formula-text.webp) |

<a id="sample-formula"></a>

## formula.png

元の解像度：448 × 64；空でない結果 **5 / 6** 件を表示。

[元画像](../../testdata/paddleocr/formula.png) · [JSON](../assets/paddleocr-results/formula.json)

| 元画像 + テキスト枠 | 元画像 + 青い認識文字 |
|---|---|
| [![formula.png — 元画像 + テキスト枠](../assets/paddleocr-results/formula-boxes.webp)](../assets/paddleocr-results/formula-boxes.webp) | [![formula.png — 元画像 + 青い認識文字](../assets/paddleocr-results/formula-text.webp)](../assets/paddleocr-results/formula-text.webp) |

<a id="sample-medal_table"></a>

## medal_table.png

元の解像度：550 × 345；空でない結果 **96 / 96** 件を表示。

[元画像](../../testdata/paddleocr/medal_table.png) · [JSON](../assets/paddleocr-results/medal_table.json)

| 元画像 + テキスト枠 | 元画像 + 青い認識文字 |
|---|---|
| [![medal_table.png — 元画像 + テキスト枠](../assets/paddleocr-results/medal_table-boxes.webp)](../assets/paddleocr-results/medal_table-boxes.webp) | [![medal_table.png — 元画像 + 青い認識文字](../assets/paddleocr-results/medal_table-text.webp)](../assets/paddleocr-results/medal_table-text.webp) |

<a id="sample-seal"></a>

## seal.png

元の解像度：640 × 640；空でない結果 **6 / 10** 件を表示。

[元画像](../../testdata/paddleocr/seal.png) · [JSON](../assets/paddleocr-results/seal.json)

| 元画像 + テキスト枠 | 元画像 + 青い認識文字 |
|---|---|
| [![seal.png — 元画像 + テキスト枠](../assets/paddleocr-results/seal-boxes.webp)](../assets/paddleocr-results/seal-boxes.webp) | [![seal.png — 元画像 + 青い認識文字](../assets/paddleocr-results/seal-text.webp)](../assets/paddleocr-results/seal-text.webp) |

<a id="sample-table"></a>

## table.jpg

元の解像度：551 × 132；空でない結果 **12 / 12** 件を表示。

[元画像](../../testdata/paddleocr/table.jpg) · [JSON](../assets/paddleocr-results/table.json)

| 元画像 + テキスト枠 | 元画像 + 青い認識文字 |
|---|---|
| [![table.jpg — 元画像 + テキスト枠](../assets/paddleocr-results/table-boxes.webp)](../assets/paddleocr-results/table-boxes.webp) | [![table.jpg — 元画像 + 青い認識文字](../assets/paddleocr-results/table-text.webp)](../assets/paddleocr-results/table-text.webp) |

<a id="sample-textline"></a>

## textline.png

元の解像度：514 × 64；空でない結果 **1 / 1** 件を表示。

[元画像](../../testdata/paddleocr/textline.png) · [JSON](../assets/paddleocr-results/textline.json)

| 元画像 + テキスト枠 | 元画像 + 青い認識文字 |
|---|---|
| [![textline.png — 元画像 + テキスト枠](../assets/paddleocr-results/textline-boxes.webp)](../assets/paddleocr-results/textline-boxes.webp) | [![textline.png — 元画像 + 青い認識文字](../assets/paddleocr-results/textline-text.webp)](../assets/paddleocr-results/textline-text.webp) |

<a id="sample-textline_rot180"></a>

## textline_rot180.jpg

元の解像度：429 × 48；空でない結果 **1 / 1** 件を表示。

[元画像](../../testdata/paddleocr/textline_rot180.jpg) · [JSON](../assets/paddleocr-results/textline_rot180.json)

| 元画像 + テキスト枠 | 元画像 + 青い認識文字 |
|---|---|
| [![textline_rot180.jpg — 元画像 + テキスト枠](../assets/paddleocr-results/textline_rot180-boxes.webp)](../assets/paddleocr-results/textline_rot180-boxes.webp) | [![textline_rot180.jpg — 元画像 + 青い認識文字](../assets/paddleocr-results/textline_rot180-text.webp)](../assets/paddleocr-results/textline_rot180-text.webp) |

出典、固定コミット、SHA-256 は[出典マニフェスト](../../testdata/paddleocr/manifest.json)を参照してください。画像部分には上流の [Apache-2.0 ライセンス](../../testdata/paddleocr/UPSTREAM_LICENSE)が適用されます。

[生成マニフェスト](../assets/paddleocr-results/manifest.json) · [再現方法](../assets/ocr-results/README.md) · [メイン README](../../README.ja.md)
