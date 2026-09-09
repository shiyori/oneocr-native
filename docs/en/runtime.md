# Existing ONNX Runtime

[简体中文](../zh-CN/runtime.md) | [English](../en/runtime.md) | [日本語](../ja/runtime.md)

Normal integration does not require a `runtime` argument. Native SDKs discover a compatible library already loaded by the application, then check bundled and installed resources. The setup command downloads the managed ORT 1.29.0 only when a usable local runtime is absent.

The native bridge requests C API 26 and creates CPU model sessions to verify required operators. ORT 1.26 and 1.29 are supported; other 1.x versions must provide the same API and model capabilities. The SDK does not depend on `onnxruntime_go` and does not raise the host's Go binding version.

An engine owns its sessions and its own runtime/environment reference. Closing it releases those references without changing the host's global options, replacing its runtime or destroying its other sessions. All API objects come from the same loaded library. Sessions for unrelated host models are not shared.

If several different ORT libraries are already loaded, the host must explicitly select the intended library through advanced configuration. An incompatible loaded library produces an error; OneOCR does not silently load a second copy or upgrade it.

Python uses the environment's compatible `onnxruntime` module, including GPU distributions, while selecting the CPU provider for OneOCR sessions. The core wheel has no hard dependency on the CPU distribution. The Android core AAR similarly reuses the app's runtime; ARM64 CPU sessions apply `mlas.disable_kleidiai=1` for compatibility.

An offline complete package carries the managed runtime and required redistributable files. Core packages use the same preparation command as complete ones. See [installation](installation.md).

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)
