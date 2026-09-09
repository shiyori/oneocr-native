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

推送与 `version.json` 一致的版本标签（例如 `v0.1.0`）会自动执行 `release-publish`：构建 Android 完整/Core AAR、通用 Python 包和可选 Linux 包，运行 Linux 原生 ABI 冒烟测试，检查制品内容、许可证和校验和，再上传草稿并核对远端 SHA-256，最后发布为正式版并设为 Latest。每个平台的构建记录绑定同一源码提交，常规 CI 必须通过。下载文档默认使用 `releases/latest` 和 `latest/download`。

扩展验证通过 Actions 手动运行 `release-build`（选择 `main`），或执行 `gh workflow run release-build.yml --ref main`。它包含完整 OCR 对照、多运行库兼容、Python 多版本消费者和 Android 模拟器测试，不是常规 CI 或正式发布的前置任务。需要保留这些报告时，成功后运行 `python scripts/publish_release.py collect --run-id RUN_ID --output ASSETS --reports REPORTS`。发布失败可从 `main` 手动运行 `release-publish`，将 `tag` 输入设为原版本标签；流程使用当前发布脚本处理原标签源码，保留 Go 模块标签不变。已公开且内容不同的制品不会被覆盖。 若构建已经成功、仅发布步骤失败，可额外填写 `build_run_id` 复用原运行的产物，只重试校验与发布。


[模型包格式](model-format.md)

Python 构建需要 `uv`。开发模型转换工具时，使用 `python -m pip install "./python[conversion]"` 安装可选依赖；普通 wheel 安装不需要这些工具。

Release 只收集 Android AAR、通用 Python wheel/sdist、共享资源及可选 Linux 包。Windows/macOS 的上述桌面构建命令仅供本地 C/C++ 开发，不发布其平台包。Linux 可省略 `--wheel-only` 生成 Python 离线包。Go 模块和命令通过 Go 工具链安装；`scripts/verify_go.py` 使用空缓存验证 `go get` / `go install`，无需桌面 SDK。

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)
