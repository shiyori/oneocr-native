# 高频识别与可选加速

默认仍使用原始模型和 ONNX Runtime CPU。Go SDK、CLI、C/C++ 可显式选择 CUDA；旧 `coreml`、`directml` 配置在默认回退策略下恢复原模型 CPU，严格策略明确报错；后端注册成功并不保证主要计算进入 GPU，也不保证更快。加速模型是独立的实验副本，不覆盖 `.ocrpack` 或原资源。

## 配置与回退

```go
config := oneocr.Config{
    ModelPath: "models/oneocr-cjk-en.ocrpack",
    Backend: oneocr.BackendCPU,
    Threads: 2,
    Fallback: oneocr.FallbackCPU,
    CacheDir: "/path/to/oneocr-cache",
}
engine, err := oneocr.Open(config)
if err != nil { return err }
defer engine.Close()
if err = engine.Warmup(ctx); err != nil { return err }
// Reuse this engine for subsequent frames; use a representative image once
// before accepting traffic to warm the actual input shapes.
result, err := engine.Recognize(ctx, frame, oneocr.Options{})
```

| 字段 | 默认与含义 |
|---|---|
| `Backend` | `cpu`；可选 `cuda`；旧 `coreml`、`directml` 仅作兼容，默认恢复 CPU |
| `DeviceID` | `0`；CUDA 的设备索引 |
| `Fallback` | `cpu`：注册、建会话或原生 Run 失败时，允许该阶段改用原模型 CPU 会话；`error` 返回错误 |
| `StageBackends` | 可单独覆盖 `detector`、`classifier`、`recognizer/CJK`、`recognizer/Latin` 等阶段 |
| `CoreMLComputeUnits` | 仅保留旧配置兼容；不再启动 CoreML |
| `CacheDir` | 保留旧配置兼容；不再生成 CoreML 编译缓存 |
| `AdaptationDir` | 空；显式使用转换工具生成的 `adaptation.json` 和模型 |
| `ProfilingDir` | 空；设置后收集 ORT kernel profile，Close 后完成统计 |
| `ShapeCacheSize` | 默认 `0` 关闭；`1..4` 限制每阶段按输入尺寸复用的会话数量 |
| `CharacterClasses` | 默认 `han,kana,hangul,latin,digits`，限制 CJK/Latin 解码；空格、标点、符号保留 |

`FallbackError` 禁止整个会话回退，不禁止 ORT 将图中不支持的算子分配给 CPU。取消和解码校验错误不触发推理重试。适配文件的来源或摘要不符直接报错，不能把损坏文件当作设备兼容问题。

字符类别用于约束候选字符，不判断文本属于哪种语言。共享汉字无法据此区分中日文；`latin` 包括模型字表内的拉丁字母，`digits` 包括其数字。扩展包和其他字表继续作为高级参考，正式验证重点为中日韩英数字。限定字符集不能让模型可靠地识别本来不支持的图像。

## 连续与并发调用

- 复用 Engine。`Warmup` 加载包内识别器，并使用小尺寸有效输入初始化各阶段；实际尺寸的首帧仍可能触发设备编译或资源分配。
- 同一个 Engine 的调用串行执行，context 包含等待 Engine 的时间。`Close` 等待活动调用结束，之后识别返回 `ErrClosed`。
- 偶发并发可以使用两个长期存活的独立 Engine，通常先从每个 Engine 一个 CPU 线程开始测试。独立会话会增加内存，不能把 ORT 线程数乘以请求并发无限放大。
- [stream 示例](../examples/stream/main.go) 使用默认 1、最多 4 个 Engine，以及 `2 × workers` 的有界任务队列；输入为逐行图片路径，输出 NDJSON 带请求 ID，允许乱序完成。持续截图应用应直接调用 `Recognize`/`RecognizeRGB`，避免磁盘往返。
- 识别器在 ORT 输出张量有效期内直接解码，避免复制整个概率矩阵；张量内容不会泄漏到返回对象。检测器和分类器仍返回生命周期独立的内部数据。

重复截图尺寸可设置 `ShapeCacheSize: 2`（CLI `--shape-cache 2`），对加速阶段按输入尺寸创建固定形状会话并做 LRU 淘汰。首次遇到尺寸可能编译较久，context 会在创建后再次检查；不能承诺硬性实时 deadline。原有固定 batch/channel/height 不会被改写，权重不变。缓存命中、淘汰及每个尺寸的实际会话配置见 `ShapeSessions`。阶段顶层 Registered 表示模板注册，尺寸会话的回退看子项。

容量限制针对存活会话；旧 CoreML 磁盘编译缓存不再使用。尺寸缓存的创建和淘汰成本也必须计入混合尺寸负载；不能从固定尺寸命中结果推断通用加速收益。

DirectML 已停用，不再注册其执行提供程序。此前验证遵循其顺序执行、禁用 memory pattern 和同一 session 的 Run 串行限制。[官方限制](https://onnxruntime.ai/docs/execution-providers/DirectML-ExecutionProvider.html#configuration-options)

## 独立模型转换

转换只在开发期使用 Python，不给 Go/C++/Android 增加 Python 运行依赖。先安装本仓库 Python 工具，再从模型包导出标准目录：

```bash
python -m pip install ./python
oneocr unpack --model models/oneocr-cjk-en.ocrpack --directory /path/to/source-bundle
oneocr-native adapt --bundle /path/to/source-bundle \
  --directory /path/to/cuda-candidate --backend cuda
```

输出目录必须不存在。清单关联原容器摘要、每个源模型摘要、转换模型摘要及输出接口；修改权重或更换包后需要重新转换。GPU 运行时和依赖库需另行提供，转换工具不会安装 CUDA、cuDNN 或 GPU 驱动。

- **CoreML**：模型适配已淘汰，不再提供转换入口，旧 CoreML 适配清单明确报错。后端枚举保留兼容；默认 `FallbackCPU` 下真正恢复原模型 CPU，诊断保留请求后端及退休原因，`FallbackError` 明确报错。
- **CUDA**：检测器保留原量化算子；识别器将 `DynamicQuantizeLSTM` 转为标准 LSTM，处理转置权重和 i/o/f/c 门顺序。
- **DirectML**：已因正确性与性能验证结果淘汰；默认 `FallbackCPU` 恢复原模型 CPU，`FallbackError` 报错。旧开发转换工具的产物不再启用 DirectML 执行。
- 分类和识别的默认 `--quantization grid` 保留激活舍入及裁剪。`--quantization relaxed` 仅保留裁剪，是更激进的数值实验。裁剪不能随意删除：量化边界可能承担 ReLU 行为。

即使保留量化网格，浮点累加和标准 LSTM 也可能改变结果，必须逐例回归。不能因为 ONNX checker 或 CPU 加载通过就宣称 GPU 可用。某阶段候选不合格时，例如 `StageBackends: map[string]oneocr.Backend{"detector": oneocr.BackendCPU}`，可保留该阶段原模型。

### 已移除整数格点检测图

`v2-detector-integer-grid` 和 `v2.1-detector-integer-grid` 已移除。它们曾通过本机检测一致性验证，
但 CoreML 与 Android 的性能明显落后于原量化 CPU；按方案淘汰要求，不再生成或加载任何后端的这类候选。
SDK 读取旧清单时明确返回 `retired detector integer-grid adaptation`，不会悄悄回退或加载旧缓存。

检测器使用 `v3-original-quantized-detector`：仅裁剪无关输出、保持 18 个公共浮点输出，
保留原 QLinearConv、QLinearAdd、QLinearSigmoid 和量化边界。检测器的 `grid`/`relaxed` 均不再展开量化计算，
也不会恢复已知存在检测差异的 v1 浮点转换。保留原模型不代表其主要计算能进入 GPU，必须核对实际执行分区。

历史资料显示，CoreML 原整数格点图被拆成大量 CPU/CoreML 分区；Android 新图的浮点 Conv 成本远高于原量化 Conv，
Cast、Gather、Add 和布局转换进一步增加开销。旧实现、精度回归捕获、逐算子 profile 和机器报告保存在仓库外，
不再把已移除方案作为可选加速功能维护。当前回归检查原量化算子保留、输出一致及旧配方拒绝加载。

Android 模拟器上原模型的 48 个卷积均为 UINT8 激活、INT8 权重、UINT8 输出；ORT 1.29 XNNPACK 的
量化卷积仅接管全 UINT8 或全 INT8，明确不支持该混合组合，因此原图只把池化交给 XNNPACK。
[ORT 1.29 支持条件](https://github.com/microsoft/onnxruntime/blob/v1.29.0/onnxruntime/core/providers/xnnpack/nn/conv_base.cc#L220-L254)
旧整数格点图把量化张量展开为 FP32 并改用浮点卷积；本模拟器 CPU profile 中 Conv 耗时约为原量化 Conv 的
6.2 倍，另有约 26.7% 的 kernel 时间用于其他算子。这些 profile 用于解释开销，正式平均耗时来自关闭 profiling 的独立计时。


验收以同机原 CPU 为参照：文字、框数量和方向相同，角点误差不超过 0.1 像素，分数误差不超过 `1e-4`。
正式性能测试关闭 profiling，按完整六图循环分别测试单/双实例三轮；每轮至少 30 秒，端到端平均耗时每轮均比同机原始 CPU 模型降低至少 20%
（候选平均耗时 ≤ 原 CPU 的 0.8，约至少 1.25 倍速度），才认定该调用模式有可重复收益。单、双实例分别验收；
候选自身 CPU 或旧 GPU 版本不能充当性能基线。冷启动、P50/P95、内存和缓存失效成本单独报告。

加速阶段拖慢整体处理时恢复原模型 CPU；最终启用组合必须达到 20% 的端到端收益门槛。该选择由实际平台验收结果确定，
使用 `StageBackends` 保持 CPU 阶段；不能让被判定更慢的路径继续作为默认加速执行。

先根据算子支持、实际分区和转换成本判断是否有明确加速依据；没有合理提速预期的方案不展开测试。
有依据的方案先短测，只有结果显示有希望达到门槛才投入完整验收。正确性失败或短测明显慢于 CPU 即淘汰，不为否定方案补足三轮。

Windows RTX 5080、ORT 1.29 / DirectML 1.15.4 的原图分阶段验证中，检测器完整 OCR 仅 37/42 图文字相同，
框数量 40/42 相同，最大角点误差 32.006 像素、分数误差 0.04799；原 Latin 仅 40/42 图文字相同。
分类器和原 CJK 的裁剪语义及 42 图文字通过，但短测完整 OCR 单实例平均耗时分别为 216.061、165.102 ms，
同机官方原 CPU 为 140.937 ms；双实例也更慢。没有应保留的 DirectML 阶段，SDK 已实际恢复 CPU 处理。

### 紧凑输出

```bash
oneocr-native adapt --bundle /path/to/source-bundle \
  --directory /path/to/compact-candidate --backend cpu --compact-output
```

仅在识别器末端添加字符掩码、ArgMax 及完整的 NaN/Inf 检查，保留原 LogSoftmax。SDK 按当前字符类别传入 `allowed_tokens`，返回帧 ID 和 `invalid` 标志；CTC 重复/blank/组合字符处理仍由 SDK 完成。该输出协议目前用于 Go/C/C++，Python 原有识别 API 仍消费完整概率输出。

## CLI 与诊断

```bash
oneocr recognize --model models/oneocr-cjk-en.ocrpack \
  --backend cpu --warmup --format json image.png

oneocr recognize --model models/oneocr-cjk-en.ocrpack \
  --backend cuda --device 0 --fallback error --cpu-stages detector \
  --adaptation-dir /path/to/cuda-candidate \
  --profile-dir /path/to/profiles --diagnostics /path/to/new-diagnostics.json image.png
```

使用 `--characters han,kana,latin,digits` 等选择字符类别。`--diagnostics` 不覆盖已有文件。高频应用使用长期存活的 SDK 或 stream 示例；反复启动单图 CLI 会重复加载模型。

`engine.Diagnostics()` 在使用中和 Close 后均可调用。每个阶段分别报告：请求后端、已注册后端、实际模型摘要、实验转换、回退原因、运行次数与耗时。启用 profiling 并 Close 后，另有实际执行 provider 的 kernel 事件及耗时。`ExecutionMeasured=false` 时不能推断实际分配；历史 CoreML 事件也不能单独证明 GPU/ANE 硬件类型，旧报告需结合 compute plan；当前版本不再创建 CoreML 会话。

profiling 用于短时诊断，持续生产运行应关闭，以免 trace 本身增加开销和内存。核对算子分配与持续吞吐应分开进行。

## 验证边界

CPU 保持默认。CoreML 加速已停用并恢复 CPU 默认处理；CUDA 需要匹配 CUDA/cuDNN 的 NVIDIA 环境，DirectML 需要 Windows/DX12 真机。编译、模型 CPU 回归、实际后端接管和稳定提速是四项独立结果，缺失真机验证的后端不标为验收通过。

性能测试应包含代表性实际截图、变化尺寸、单流/双调用方和冷启动；本轮按约两分钟的短时连续调用验证，比较同机 CPU 的 P50/P95、吞吐和 RSS/显存。短时生成样例不代表生产容量或通用准确率。本轮接入实测脚本、详细 trace 和机器报告留在仓库外；仓库保留转换与生命周期回归测试。

Independent detection and cropped-line recognition: [API guide](STAGES.md). Go, CLI, C/C++ and Python expose separate entry points.

历史 v1 CoreML 浮点检测器候选在新增 PaddleOCR 图片回归中与 CPU 对照出现文本和框差异，未满足“无新增识别错误”的验收条件。v2/v2.1 曾通过一致性验证，但现已移除其实现和加载入口。部分 720p 合成图上的短时提速不能代表通用准确率或稳定生产性能。Windows CUDA v1 已执行但候选未通过一致性验收；DirectML 已以隔离构建的 ORT 1.29 完成初步推理，完整阶段正确性与性能验收需单独记录。

CUDA v1 浮点候选在 CPU 对照中也存在扩展参考样例的新增检测行。DirectML 的公开 ORT 1.24.4 包无法提供 SDK 所需的 API 29；现已从 ORT v1.29.0 源码构建匹配运行库，初步检测器捕获输入的 18 个输出精确一致。标准 LSTM 转换有额外数值误差，不能仅因全部算子进入 DML 就采用。


### 历史 v1 平台结果与运行时边界

- 原量化检测器在 macOS CoreML 与 CPU 的当前独立检测样例上保持一致，但 profile 显示主要计算仍由 CPU 执行。浮点转换候选在 CPU 上已改变部分框；同一候选切换 CoreML 又有额外差异。尺寸缓存没有解决这些几何差异。
- Windows x64 的 CPU Go/C++/Python 及独立检测/单行识别接口已实际检查。CUDA 也取得了真实 kernel 执行记录；原图仅加速检测器时，本轮完整 OCR 输出与 CPU 一致，但大部分检测计算仍在 CPU。原图全阶段 CUDA 和转换候选存在文字差异，不能推广为等价加速。
- 本次 Windows ORT 1.29 CUDA 二进制实际需要 CUDA 13 的 cuBLAS/cudart 和 cuDNN 9。隔离测试使用 cuBLAS 13.0.2.14、cudart 13.0.96、cuDNN 9.13.0.50；这描述已测试组合，不代表所有 ORT 构建的依赖相同。缺库时必须区分显式报错与已记录原因的 CPU 回退。
- 本次取得的官方 DirectML ORT 1.24.4 仅提供至 API 24，不能被当前 API 29 绑定初始化。需要兼容的运行时或独立兼容性工作，不能仅通过降低版本检查绕过 ABI 要求。
- 当前隔离 DirectML 构建使用 ORT v1.29.0（源码 `2e2543fbe9fae542f921d47a72d21d5a4ef0b710`）、DirectML 1.15.4、VS2022/MSVC 19.44、Windows SDK 10.0.26100.0；从官方源码以 `--use_dml --build_shared_lib --config Release` 构建。`onnxruntime.dll` 与 `DirectML.dll` 配套使用，保持与 CUDA 运行库隔离。最小矩阵模型及冻结 SDK 已在 RTX 5080 上记录实际 DML 执行；这属于兼容性验证，不替代 42 图及三轮性能验收。
- 同一提交和输入的 CPU OCR 文字在 macOS/Windows 的复杂图片上也可能有差异；跨平台一致性需逐图核对。机器测量与逐图报告继续保存在仓库外。
