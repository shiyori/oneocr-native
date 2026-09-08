# PaddleOCR test images

Source: [PaddlePaddle/PaddleOCR tests/test_files](https://github.com/PaddlePaddle/PaddleOCR/tree/2661c7c0ef5c613e8f93c6e93b2e052399f0f854/tests/test_files), pinned commit `2661c7c0ef5c613e8f93c6e93b2e052399f0f854`. Original images are unchanged; file URLs, sizes and SHA-256 values are in `manifest.json`. The upstream Apache-2.0 repository license is retained as `UPSTREAM_LICENSE`; these third-party images are not relicensed as this project's MIT code.

The `720p-*.png` derivatives preserve aspect ratio, downscale only when necessary, and add white margins to 1280×720. No upscaling or stretching is applied. Derivative hashes are recorded separately.

No official text ground truth was supplied alongside these files or in their directly associated tests. They support behavior and CPU/candidate comparisons; agreement is not an accuracy score. Books, tables, formulas and seals include content outside ordinary horizontal OCR coverage.
