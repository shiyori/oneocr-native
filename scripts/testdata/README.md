# Release smoke-test fixture

`cjk-line.png` is the first detector region from `../../testdata/CJK.png`, rectified by the Go engine with the default CJK/English model and ONNX Runtime 1.29.0. Its expected text is `你好世界 日本語テスト 한국어 123`.

The release smoke test uses the original image for `recognize` and `detect`, and this cropped line for `recognize-line`, which does not perform detection. This file stays outside the main `testdata/` corpus so the 42-image reference comparison set remains unchanged.
