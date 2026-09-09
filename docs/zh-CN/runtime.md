# 已有 ONNX Runtime

[简体中文](../zh-CN/runtime.md) | [English](../en/runtime.md) | [日本語](../ja/runtime.md)

常规接入无需 `runtime` 参数。原生 SDK 优先发现应用已经加载的兼容运行库，再查找随包和已安装资源。本地没有可用运行库时，准备命令才下载固定的 ORT 1.29.0。

原生适配层请求 C API 26，并通过创建 CPU 模型会话确认所需算子。支持 ORT 1.26 和 1.29；其他 1.x 版本也需提供相同 API 与模型能力。SDK 不依赖 `onnxruntime_go`，不会抬高宿主的 Go 绑定版本。

Engine 持有自己的会话和运行库/环境引用。关闭时释放这些引用，不修改宿主全局选项、不替换运行库，也不销毁宿主其他模型会话。所有 API 对象来自同一个运行库；宿主其他模型的 Session 不会被共用。

如果进程已加载多个不同的 ORT 库，宿主需要通过高级配置明确选定目标。已加载的运行库不兼容时会返回错误，不会自动另加载一份或升级它。

Python 复用当前环境中兼容的 `onnxruntime` 模块，包括 GPU 发行包，但 OneOCR 会话指定 CPU provider。精简 wheel 不强制依赖 CPU 发行包。Android 精简 AAR 同样复用应用运行库；ARM64 CPU 会话设置 `mlas.disable_kleidiai=1` 以兼容相应设备。

完整离线包携带固定运行库及所需可再分发文件；精简包使用相同的准备命令。详见[下载与安装](installation.md)。

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)
