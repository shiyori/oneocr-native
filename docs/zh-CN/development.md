# 开发指南

[简体中文](../zh-CN/development.md) | [English](../en/development.md) | [日本語](../ja/development.md)

本页用于构建 SDK 本身。应用接入请从 [Release 下载](installation.md)开始。

Go 根包只保留公开入口（`oneocr.go`）、类型别名（`types.go`）和包说明（`doc.go`）。识别引擎、模型解析、安装逻辑及内部测试位于 `internal/engine/`；ONNX Runtime 绑定位于 `internal/ort/`，安装锁位于 `internal/installlock/`。命令入口保留在 `cmd/`。应用仍然导入 `github.com/shiyori/oneocr-native`。

普通 push/PR 只运行三平台 Go 单元测试与 vet（Linux 同时检查数据竞争）、一套 Python 单元测试及版本/模型元数据检查。不会执行完整 OCR 对照、启动 Android 模拟器或构建发布包；新提交会取消同分支尚未完成的常规检查。完整验证仅在手动触发发布构建时运行。

```sh
git clone https://github.com/shiyori/oneocr-native.git
cd oneocr-native
python scripts/version.py --check
go test ./...
go vet ./...
```

版本统一维护在 `version.json`，修改后运行 `python scripts/version.py`。原生版本为 `0.1.0`，Python 版本为 `0.1.0`。

在目标系统及架构构建桌面制品：

```sh
python scripts/build_sdk.py --target desktop --output dist/release
python scripts/build_python.py --wheel-only --output dist/release
```

构建工具下载固定的上游运行库，打包默认模型，生成完整/精简归档并保留第三方许可证。Windows 需要 C 编译器、Visual Studio C++ 工具和可再分发文件；macOS 需要 Apple 命令行工具；Linux 以 Ubuntu 22.04 为构建基线。

Android 需要 Android SDK、NDK 27+、JDK 17+ 和 CMake：

```sh
python scripts/build_sdk.py --target android --output dist/release --android-sdk /path/to/android-sdk --android-ndk /path/to/ndk
```

Release 工作流在仓库外验证消费者接入，同时执行运行库兼容检查、Python 3.11–3.13 测试、Android 两个 ABI 的实际 OCR，以及制品、许可证、链接校验。完整制品全部通过后才发布正式版本。研究资料和保留的开发模型不进入 Release。

在 Actions 页面手动触发 `release-build`（选择 `main`），或执行 `gh workflow run release-build.yml --ref main`。该耗时工作流不会随 push/PR 自动运行，也不会自动发布。通过后执行 `python scripts/publish_release.py collect --run-id RUN_ID --output ASSETS --reports REPORTS` 下载并验证这批制品。使用 `scripts/verify_android.py` 在 ARM64 设备上验证这些 AAR：完整包，以及宿主 ORT 1.26.0、1.29.0 的 Core 包。带注释的版本 tag 保存 JSON 回执，包含 `schema: oneocr.release-receipt.v1`、`version`、`commit`、`build_run_id` 和按完整包/1.26/1.29 顺序排列的三个 `android_arm64` 报告对象。tag 工作流核对所有报告与制品哈希，上传草稿并校验上传结果后发布，不在测试后重新构建。

[模型包格式](model-format.md)

Python 构建需要 `uv`。开发模型转换工具时，使用 `python -m pip install "./python[conversion]"` 安装可选依赖；普通 wheel 安装不需要这些工具。

Release 只收集 Android AAR、通用 Python wheel/sdist、共享资源及可选 Linux 包。Windows/macOS 的上述桌面构建命令仅供本地 C/C++ 开发，不发布其平台包。Linux 可省略 `--wheel-only` 生成 Python 离线包。Go 模块和命令通过 Go 工具链安装；`scripts/verify_go.py` 使用空缓存验证 `go get` / `go install`，无需桌面 SDK。

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)
