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

`version.json` is the version source. Run `python scripts/version.py` after changing it. The native version is `0.1.1` and the Python version is `0.1.1`.

Build the complete Linux runtime package and universal Python distribution on the target Linux architecture:

```sh
python scripts/build_linux.py --output dist/release
python scripts/build_python.py --wheel-only --output dist/release
```

The build tools fetch pinned upstream runtimes, package the default model, generate complete runtime packages and retain third-party licenses. Windows requires a C compiler plus the Visual Studio C++ tools and redistributable files. macOS requires Apple command-line tools. Linux builds use an Ubuntu 22.04 baseline.

Android builds require the Android SDK, NDK 27+, JDK 17+ and CMake:

```sh
python scripts/build_sdk.py --target android --output dist/release --android-sdk /path/to/android-sdk --android-ndk /path/to/ndk
```

Pushing a version tag matching `version.json` (for example `v0.1.2`) automatically runs `release-publish`: build Android full/Core AARs, the universal Python distribution and one complete Linux package per architecture; verify relocated offline Linux OCR and installation from official dependencies; audit contents, licenses and checksums; upload a draft and verify remote SHA-256 digests; then publish a stable release and mark it Latest. Build records bind every platform's artifacts to the same source commit, and regular CI must pass. Download documentation defaults to `releases/latest` and `latest/download`.

Extended validation is available by manually dispatching `release-build` on `main`, or running `gh workflow run release-build.yml --ref main`. It includes full OCR comparisons, runtime compatibility, multiple Python consumer versions and Android emulator tests. It is not required by ordinary CI or formal publication. To retain its reports, run `python scripts/publish_release.py collect --run-id RUN_ID --output ASSETS --reports REPORTS` after it succeeds. Retry failed publication by dispatching `release-publish` from `main` with the original `tag` input. Current publication tooling builds the original tagged source without moving the Go module tag; already published assets with different contents will not be overwritten. If builds succeeded and only publication failed, set `build_run_id` to reuse those artifacts and retry only verification and publication.


For a pre-release check, dispatch `release-publish` with `tag=main` and `publish=false` to build and audit only. Once it passes, create the version tag on the same commit with `OneOCR-Build-Run: RUN_ID` in its annotation. Tag publication reuses those exact artifacts without rebuilding.

[Model package format](model-format.md)

Python builds require `uv`. For model-conversion development, install the optional tools with `python -m pip install "./python[conversion]"`; ordinary wheel users do not need them.

Releases recommend the complete Android AAR and Linux runtime packages, alongside the universal Python wheel/sdist, model resources and advanced Android Core AAR. Separate runtime archives, Linux SDK/Core and Linux Python offline bundles are no longer published. Go/Python code integrations use the installer to prepare complete dependencies.

C/C++ development can build a complete local development kit from the repository on its target platform:

```sh
python scripts/build_sdk.py --target desktop --output dist/native-dev
```

This output is for local development and is not uploaded to Releases. Go does not require it; `scripts/verify_go.py` checks `go get` / `go install` from empty caches.

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)
