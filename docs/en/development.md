# Development

[简体中文](../zh-CN/development.md) | [English](../en/development.md) | [日本語](../ja/development.md)

This page is for building the SDK itself. Application integration starts with [Release downloads](installation.md).

The root Go package contains public entry points (`oneocr.go`), type aliases (`types.go`) and package documentation (`doc.go`). OCR, model parsing, installation and internal tests live in `internal/engine/`; runtime bindings live in `internal/ort/`, and installation locks in `internal/installlock/`. Commands remain in `cmd/`. Applications still import `github.com/shiyori/oneocr-native`.

Ordinary pushes/PRs run Go unit tests and vet on three platforms (with race detection on Linux), one Python unit suite, and version/model metadata checks. They do not run full OCR comparisons, start Android emulators or build release packages. A newer push cancels unfinished ordinary checks on the same branch. Full validation runs only when the release build is manually dispatched.

```sh
git clone https://github.com/shiyori/oneocr-native.git
cd oneocr-native
python scripts/version.py --check
go test ./...
go vet ./...
```

`version.json` is the version source. Run `python scripts/version.py` after changing it. The native version is `0.1.0` and the Python version is `0.1.0`.

Build desktop SDKs on the target architecture:

```sh
python scripts/build_sdk.py --target desktop --output dist/release
python scripts/build_python.py --wheel-only --output dist/release
```

The build tools fetch pinned upstream runtimes, package the default model, generate complete/core archives and retain third-party licenses. Windows requires a C compiler plus the Visual Studio C++ tools and redistributable files. macOS requires Apple command-line tools. Linux builds use an Ubuntu 22.04 baseline.

Android builds require the Android SDK, NDK 27+, JDK 17+ and CMake:

```sh
python scripts/build_sdk.py --target android --output dist/release --android-sdk /path/to/android-sdk --android-ndk /path/to/ndk
```

The release workflow runs consumer checks outside the checkout, runtime compatibility tests, Python 3.11–3.13 tests, both Android ABI OCR checks and archive/license/link validation. Only a complete passing asset set may publish the release. Research files and the reserved development model are excluded from release assets.

Manually dispatch `release-build` from Actions on `main`, or run `gh workflow run release-build.yml --ref main`. This long-running workflow does not start on push/PR and does not publish automatically. After it succeeds, `python scripts/publish_release.py collect --run-id RUN_ID --output ASSETS --reports REPORTS` downloads and verifies the exact artifacts. Run `scripts/verify_android.py` against those AARs on an ARM64 device: full package, then Core with host ORT 1.26.0 and 1.29.0. The annotated release tag contains a JSON receipt with `schema: oneocr.release-receipt.v1`, `version`, `commit`, `build_run_id`, and those three report objects in `android_arm64` (full/1.26/1.29 order). The tag workflow verifies all reports against the artifact hashes, uploads a draft, checks the uploaded hashes and publishes. It does not rebuild after testing.

[Model package format](model-format.md)

Python builds require `uv`. For model-conversion development, install the optional tools with `python -m pip install "./python[conversion]"`; ordinary wheel users do not need them.

Releases collect Android AARs, the universal Python wheel/sdist, shared resources and optional Linux packages. Desktop builds on Windows/macOS are only for local C/C++ development and are not published. On Linux, omit `--wheel-only` to build the Python offline bundle. Go modules and commands install through the Go toolchain; `scripts/verify_go.py` checks `go get` / `go install` from empty caches without a desktop SDK.

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)
