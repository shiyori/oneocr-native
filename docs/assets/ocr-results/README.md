# OCR example images

These paired images were generated using the public OneOCR v0.1.2 CLI, the default CJK/English model, and the actual inputs recorded in [manifest.json](manifest.json).

- Keep a line only when `confidence >= 0.80` and `detection_score >= 0.70`. Null recognition confidence is excluded.
- Both images retain the original pixels. The left adds detection quadrilaterals. The right places blue recognized text inside the corresponding quadrilaterals on a local, translucent light background; text is fitted without changing its content.
- Both sides use the same outer-whitespace crop and a 2× presentation scale. The manifest records that transform; original JSON coordinates remain unchanged.
- Results are raw model output, including remaining errors. Confidence is an uncalibrated CTC token score, not measured accuracy. Full, unfiltered recognition output is retained in each sample JSON.
- WebP images use lossless encoding. Fonts are used locally to render text and are not distributed with these assets.

## Regenerate

Use the repository's Python environment with Pillow, NumPy and OpenCV, an installed OneOCR command with prepared resources, and a font covering Chinese, Japanese, Korean and Latin characters:

```sh
python scripts/render_ocr_examples.py --font /path/to/cjk-font.ttf
```

Optional arguments: `--oneocr /path/to/oneocr`, `--home /path/to/installed/resources`, `--min-confidence 0.80`, and `--min-detection-score 0.70`.

After changing thresholds, regenerate the assets and update the counts and threshold captions in all three main READMEs.

## Source and license

The mixed-language image comes from [testdata/720p](../../../testdata/720p). The book, formula document and medal table are presentation derivatives of the images already maintained in [testdata/paddleocr](../../../testdata/paddleocr), pinned to PaddlePaddle/PaddleOCR commit `2661c7c0…`. Their original URLs and hashes are in the [source manifest](../../../testdata/paddleocr/manifest.json).

The PaddleOCR image portions retain their [upstream Apache-2.0 license](../../../testdata/paddleocr/UPSTREAM_LICENSE); these images are not relicensed as AGPL code. See [THIRD_PARTY_NOTICES.md](../../../THIRD_PARTY_NOTICES.md).
