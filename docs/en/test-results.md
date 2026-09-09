# PaddleOCR original-image test results

[简体中文](../zh-CN/test-results.md) | [English](../en/test-results.md) | [日本語](../ja/test-results.md)

All 9 original images in `testdata/paddleocr` were rerun with the public OneOCR v0.1.3 release and the default Chinese/Japanese/Korean/English model. Inputs retain their original dimensions, without conversion to 720p.

Paired images show nonempty results with recognition confidence ≥ **0.70** and detection score ≥ **0.70**; JSON retains all recognition output. Confidence is an uncalibrated model score. These samples have no complete manual annotations, so the counts below are not accuracy measurements.

The left image adds boxes to the original; the right overlays blue recognized text with local light backings. Both retain the full canvas and are rendered at 2× scale; JSON coordinates refer to the original image. Text is not manually corrected. Click an image to enlarge it.

| Original | Original dimensions | Nonempty results | Displayed results | Raw JSON |
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

Original dimensions: 1100 × 708; displaying **43 / 44** nonempty results.

[Original image](../../testdata/paddleocr/book.jpg) · [JSON](../assets/paddleocr-results/book.json)

| Original + text boxes | Original + blue recognized text |
|---|---|
| [![book.jpg — Original + text boxes](../assets/paddleocr-results/book-boxes.webp)](../assets/paddleocr-results/book-boxes.webp) | [![book.jpg — Original + blue recognized text](../assets/paddleocr-results/book-text.webp)](../assets/paddleocr-results/book-text.webp) |

<a id="sample-book_rot180"></a>

## book_rot180.jpg

Original dimensions: 1100 × 708; displaying **46 / 46** nonempty results.

[Original image](../../testdata/paddleocr/book_rot180.jpg) · [JSON](../assets/paddleocr-results/book_rot180.json)

| Original + text boxes | Original + blue recognized text |
|---|---|
| [![book_rot180.jpg — Original + text boxes](../assets/paddleocr-results/book_rot180-boxes.webp)](../assets/paddleocr-results/book_rot180-boxes.webp) | [![book_rot180.jpg — Original + blue recognized text](../assets/paddleocr-results/book_rot180-text.webp)](../assets/paddleocr-results/book_rot180-text.webp) |

<a id="sample-doc_with_formula"></a>

## doc_with_formula.png

Original dimensions: 816 × 1056; displaying **102 / 116** nonempty results.

[Original image](../../testdata/paddleocr/doc_with_formula.png) · [JSON](../assets/paddleocr-results/doc_with_formula.json)

| Original + text boxes | Original + blue recognized text |
|---|---|
| [![doc_with_formula.png — Original + text boxes](../assets/paddleocr-results/doc_with_formula-boxes.webp)](../assets/paddleocr-results/doc_with_formula-boxes.webp) | [![doc_with_formula.png — Original + blue recognized text](../assets/paddleocr-results/doc_with_formula-text.webp)](../assets/paddleocr-results/doc_with_formula-text.webp) |

<a id="sample-formula"></a>

## formula.png

Original dimensions: 448 × 64; displaying **5 / 6** nonempty results.

[Original image](../../testdata/paddleocr/formula.png) · [JSON](../assets/paddleocr-results/formula.json)

| Original + text boxes | Original + blue recognized text |
|---|---|
| [![formula.png — Original + text boxes](../assets/paddleocr-results/formula-boxes.webp)](../assets/paddleocr-results/formula-boxes.webp) | [![formula.png — Original + blue recognized text](../assets/paddleocr-results/formula-text.webp)](../assets/paddleocr-results/formula-text.webp) |

<a id="sample-medal_table"></a>

## medal_table.png

Original dimensions: 550 × 345; displaying **96 / 96** nonempty results.

[Original image](../../testdata/paddleocr/medal_table.png) · [JSON](../assets/paddleocr-results/medal_table.json)

| Original + text boxes | Original + blue recognized text |
|---|---|
| [![medal_table.png — Original + text boxes](../assets/paddleocr-results/medal_table-boxes.webp)](../assets/paddleocr-results/medal_table-boxes.webp) | [![medal_table.png — Original + blue recognized text](../assets/paddleocr-results/medal_table-text.webp)](../assets/paddleocr-results/medal_table-text.webp) |

<a id="sample-seal"></a>

## seal.png

Original dimensions: 640 × 640; displaying **6 / 10** nonempty results.

[Original image](../../testdata/paddleocr/seal.png) · [JSON](../assets/paddleocr-results/seal.json)

| Original + text boxes | Original + blue recognized text |
|---|---|
| [![seal.png — Original + text boxes](../assets/paddleocr-results/seal-boxes.webp)](../assets/paddleocr-results/seal-boxes.webp) | [![seal.png — Original + blue recognized text](../assets/paddleocr-results/seal-text.webp)](../assets/paddleocr-results/seal-text.webp) |

<a id="sample-table"></a>

## table.jpg

Original dimensions: 551 × 132; displaying **12 / 12** nonempty results.

[Original image](../../testdata/paddleocr/table.jpg) · [JSON](../assets/paddleocr-results/table.json)

| Original + text boxes | Original + blue recognized text |
|---|---|
| [![table.jpg — Original + text boxes](../assets/paddleocr-results/table-boxes.webp)](../assets/paddleocr-results/table-boxes.webp) | [![table.jpg — Original + blue recognized text](../assets/paddleocr-results/table-text.webp)](../assets/paddleocr-results/table-text.webp) |

<a id="sample-textline"></a>

## textline.png

Original dimensions: 514 × 64; displaying **1 / 1** nonempty results.

[Original image](../../testdata/paddleocr/textline.png) · [JSON](../assets/paddleocr-results/textline.json)

| Original + text boxes | Original + blue recognized text |
|---|---|
| [![textline.png — Original + text boxes](../assets/paddleocr-results/textline-boxes.webp)](../assets/paddleocr-results/textline-boxes.webp) | [![textline.png — Original + blue recognized text](../assets/paddleocr-results/textline-text.webp)](../assets/paddleocr-results/textline-text.webp) |

<a id="sample-textline_rot180"></a>

## textline_rot180.jpg

Original dimensions: 429 × 48; displaying **1 / 1** nonempty results.

[Original image](../../testdata/paddleocr/textline_rot180.jpg) · [JSON](../assets/paddleocr-results/textline_rot180.json)

| Original + text boxes | Original + blue recognized text |
|---|---|
| [![textline_rot180.jpg — Original + text boxes](../assets/paddleocr-results/textline_rot180-boxes.webp)](../assets/paddleocr-results/textline_rot180-boxes.webp) | [![textline_rot180.jpg — Original + blue recognized text](../assets/paddleocr-results/textline_rot180-text.webp)](../assets/paddleocr-results/textline_rot180-text.webp) |

Sources, pinned commits and SHA-256 hashes are in the [source manifest](../../testdata/paddleocr/manifest.json). Image portions retain the upstream [Apache-2.0 license](../../testdata/paddleocr/UPSTREAM_LICENSE).

[Generation manifest](../assets/paddleocr-results/manifest.json) · [Reproduce](../assets/ocr-results/README.md) · [Main README](../../README.en.md)
