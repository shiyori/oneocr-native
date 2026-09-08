# Synthetic OCR fixtures

These small images were generated for this project's tests using macOS CoreText font rendering. Ground-truth strings are in `annotations.json`; rotations, contrast and layout variants derive from the same synthetic inputs. The source snapshot's external book photograph is intentionally not included in the root fixture set.

Renderers are retained in `python/tests/render_fixtures.swift` and `python/tests/generate_variants.py`. These fixtures test basic script handling and regressions; they are not a representative accuracy benchmark for documents, handwriting or natural vertical text.

Additional fixtures: [`720p/`](720p/README.md) contains three generated mixed-text images with source text; [`paddleocr/`](paddleocr/README.md) contains nine upstream images and nine 1280×720 derivatives, with pinned source metadata. Detailed performance reports remain outside the repository.
