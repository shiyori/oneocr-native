# Third-party notices

The project source is MIT licensed. It is an unofficial learning and research implementation; the stated purpose adds no restriction to the MIT grant. No warranty is provided.

The Microsoft OneOCR model resources in `models/` are third-party materials. The project MIT license does not relicense those weights, dictionaries or original configuration. Their original rights and applicable terms remain with their owners; no separate model redistribution license was supplied with the original input. See the model directory's notices and checksums.

Native SDK distributions carry the upstream ONNX Runtime MIT license and ThirdPartyNotices, onnxruntime_go MIT license, and golang.org/x/text BSD license and patent grant in `licenses/` (AAR: `META-INF/oneocr/licenses/`).

The independent Python SDK uses separately distributed NumPy, ONNX, ONNX Runtime, OpenCV, Pillow, PyCryptodome and filelock packages. Their installed distributions retain their own licenses and notices; this wheel does not relicense or vendor those dependencies. The macOS RTL path uses system ICU; other platforms use this project's portable approximation.

PaddleOCR test images under `testdata/paddleocr/` are sourced from PaddlePaddle/PaddleOCR; see that directory’s README, pinned manifest and UPSTREAM_LICENSE for provenance and Apache-2.0 repository licensing. Generated 720p derivatives are identified in the manifest.
