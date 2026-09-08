# 高频识别与可选加速

默认仍使用原始模型和 ONNX Runtime CPU。Go SDK、CLI、C/C++ 可显式选择 CoreML、CUDA、DirectML；后端注册成功并不保证主要计算进入 GPU，也不保证更快。加速模型是独立的实验副本，不覆盖 `.ocrpack` 或原资源。

## 配置与回退

```go
config := oneocr.Config{
    ModelPath: "models/oneocr-cjk-en.ocrpack",
    Backend: oneocr.BackendCoreML,
    Threads: 2,
    Fallback: oneocr.FallbackCPU,
    CacheDir: "/path/to/oneocr-cache",
    // AdaptationDir: "/path/to/experimental-coreml-models",
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
| `Backend` | `cpu`；可选 `coreml`、`cuda`、`directml` |
| `DeviceID` | `0`；CUDA/DirectML 的设备索引，CoreML 由系统调度 |
| `Fallback` | `cpu`：注册、建会话或原生 Run 失败时，允许该阶段改用原模型 CPU 会话；`error` 返回错误 |
| `StageBackends` | 可单独覆盖 `detector`、`classifier`、`recognizer/CJK`、`recognizer/Latin` 等阶段 |
| `CoreMLComputeUnits` | `ALL`；亦可选 `CPUOnly`、`CPUAndGPU`、`CPUAndNeuralEngine` |
| `CacheDir` | 系统用户缓存目录下的 `oneocr`；CoreML 编译缓存按模型内容、ORT 版本、平台及选项隔离 |
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

容量限制针对存活会话；CoreML 磁盘编译缓存按内容和尺寸保存，全部 Engine 停止后可以清理 `CacheDir`。高频、固定画面可先验证仅检测器使用 CoreML + 尺寸缓存，分类和识别器保留 CPU；全阶段 GPU 不一定更快。

DirectML 使用顺序执行并禁用 memory pattern；同一 session 不允许多个 `Run` 同时执行，独立 Engine 使用各自 session。[官方限制](https://onnxruntime.ai/docs/execution-providers/DirectML-ExecutionProvider.html#configuration-options)

## 独立模型转换

转换只在开发期使用 Python，不给 Go/C++/Android 增加 Python 运行依赖。先安装本仓库 Python 工具，再从模型包导出标准目录：

```bash
python -m pip install ./python
oneocr unpack --model models/oneocr-cjk-en.ocrpack --directory /path/to/source-bundle
oneocr-native adapt --bundle /path/to/source-bundle \
  --directory /path/to/coreml-candidate --backend coreml
oneocr-native adapt --bundle /path/to/source-bundle \
  --directory /path/to/cuda-candidate --backend cuda
oneocr-native adapt --bundle /path/to/source-bundle \
  --directory /path/to/directml-candidate --backend directml
```

输出目录必须不存在。清单关联原容器摘要、每个源模型摘要、转换模型摘要及输出接口；修改权重或更换包后需要重新转换。GPU 运行时和依赖库需另行提供，转换工具不会安装 CUDA、cuDNN 或 GPU 驱动。

- **CoreML**：检测器默认使用下述 v2 整数网格候选；识别器保留 CPU 的量化 LSTM，将量化投影展开为 MatMul。
- **CUDA**：检测器默认使用同一 v2 配方；识别器将 `DynamicQuantizeLSTM` 转为标准 LSTM，处理转置权重和 i/o/f/c 门顺序。
- **DirectML**：优先保留原量化检测器与投影，转换不支持的量化 LSTM。
- 默认 `--quantization grid` 保留激活舍入及裁剪。`--quantization relaxed` 仅保留裁剪，是更激进的数值实验。裁剪不能随意删除：量化边界可能承担 ReLU 行为。

即使保留量化网格，浮点累加和标准 LSTM 也可能改变结果，必须逐例回归。不能因为 ONNX checker 或 CPU 加载通过就宣称 GPU 可用。某阶段候选不合格时，例如 `StageBackends: map[string]oneocr.Backend{"detector": oneocr.BackendCPU}`，可保留该阶段原模型。

### 检测器 v2 整数网格配方

CoreML/CUDA 的 `--quantization grid` 检测器使用 `v2-detector-integer-grid`。
分类器、识别器和显式 `relaxed` 模式仍使用原配方。公开检测接口仍为 18 个浮点输出，
无需修改 Go/C/C++ 调用代码；旧候选目录不会自动升级，需重新生成到新目录。

- 内部 FP32 张量表示精确的 UINT8 整数值，保留原始输入 QuantizeLinear 和输出 DequantizeLinear。
- 卷积先减 zero point，以整数值权重计算；按绝对累加上界将超出 `2^24` 的卷积拆为连续通道段。
  段结果转为 INT32 后求和并加入原 INT32 偏置，再按原 CPU 算子的 FLOAT32 比例、舍入和截断重定标。
  上界可能溢出 INT32、单通道无法安全拆分或量化连接不匹配时，转换报错。
- Add/Sigmoid 分别使用完整的 65536 项和 256 项查找表，表由转换环境的 ORT CPU 算子生成。
  **在目标平台生成候选**，并以该机器的原模型 CPU 为基准验收；不要将一个平台的表视为所有平台的参考。
- 清单记录配方、参考 ORT 版本/系统/架构、源和候选摘要以及拆分卷积数量；现有按模型内容划分的运行缓存隔离新旧模型。

该配方仍标记为 experimental/approximate：FP32 类型和累加上界不能保证设备内部没有降低精度或变换算法。
CoreML 的部分整数操作可能由 ORT CPU 分区执行；这与整个阶段失败后的 CPU 回退不同。
需核对 profile 和 CoreML compute plan，不能仅凭输出一致声称 GPU 加速成功。

验收分别比较新图 CPU、CoreML/CUDA 与同机原模型 CPU：框数量、方向和最终文字相同，
匹配框角点偏差不超过 0.1 像素，分数绝对误差不超过 `1e-4`；同时检查原始 18 个输出、
变化尺寸和缓存关闭/开启。正确性通过后才进行约两分钟单/双实例性能测试。
未达标时保留失败节点和最小复现，不以调整检测阈值或自动改用 CPU 宣称修复完成。

单元测试覆盖舍入边界、zero point、偏置、分组/步幅/填充、分段累加与查找表穷举。
`ONEOCR_DETECTOR_REGRESSION_DIR` 可指向仓库外的原始回归资料目录，运行实际首个差异节点及完整检测输出回归；
需要其中的 `formal-cjk-source/models/detection/universal.onnx` 和 `universal/input0.bin`（FLOAT32、1×3×160×1024）。
未提供资料时该测试明确跳过，不将模型或捕获张量写入测试源码。

### 紧凑输出

```bash
oneocr-native adapt --bundle /path/to/source-bundle \
  --directory /path/to/compact-candidate --backend cpu --compact-output
```

仅在识别器末端添加字符掩码、ArgMax 及完整的 NaN/Inf 检查，保留原 LogSoftmax。SDK 按当前字符类别传入 `allowed_tokens`，返回帧 ID 和 `invalid` 标志；CTC 重复/blank/组合字符处理仍由 SDK 完成。该输出协议目前用于 Go/C/C++，Python 原有识别 API 仍消费完整概率输出。

## CLI 与诊断

```bash
oneocr recognize --model models/oneocr-cjk-en.ocrpack \
  --backend coreml --adaptation-dir /path/to/coreml-candidate \
  --warmup --format json image.png

oneocr recognize --model models/oneocr-cjk-en.ocrpack \
  --backend cuda --device 0 --fallback error --cpu-stages detector \
  --adaptation-dir /path/to/cuda-candidate \
  --profile-dir /path/to/profiles --diagnostics /path/to/new-diagnostics.json image.png
```

使用 `--characters han,kana,latin,digits` 等选择字符类别。`--diagnostics` 不覆盖已有文件。高频应用使用长期存活的 SDK 或 stream 示例；反复启动单图 CLI 会重复加载模型。

`engine.Diagnostics()` 在使用中和 Close 后均可调用。每个阶段分别报告：请求后端、已注册后端、实际模型摘要、实验转换、回退原因、运行次数与耗时。启用 profiling 并 Close 后，另有实际执行 provider 的 kernel 事件及耗时。`ExecutionMeasured=false` 时不能推断实际分配；CoreML 事件也不能单独证明 GPU/ANE 硬件类型，需要 CoreML compute plan 辅助检查。

profiling 用于短时诊断，持续生产运行应关闭，以免 trace 本身增加开销和内存。核对算子分配与持续吞吐应分开进行。

## 验证边界

CPU 保持默认。CoreML 可在 macOS 实测；CUDA 需要匹配 CUDA/cuDNN 的 NVIDIA 环境，DirectML 需要 Windows/DX12 真机。编译、模型 CPU 回归、实际后端接管和稳定提速是四项独立结果，缺失真机验证的后端不标为验收通过。

性能测试应包含代表性实际截图、变化尺寸、单流/双调用方和冷启动；本轮按约两分钟的短时连续调用验证，比较同机 CPU 的 P50/P95、吞吐和 RSS/显存。短时生成样例不代表生产容量或通用准确率。本轮接入实测脚本、详细 trace 和机器报告留在仓库外；仓库保留转换与生命周期回归测试。

Independent detection and cropped-line recognition: [API guide](STAGES.md). Go, CLI, C/C++ and Python expose separate entry points.

历史 v1 CoreML 浮点检测器候选在新增 PaddleOCR 图片回归中与 CPU 对照出现文本和框差异，未满足“无新增识别错误”的验收条件。该结果不能作为 v2 的验收结论。部分 720p 合成图上的短时提速不能代表通用准确率或稳定生产性能。Windows CUDA v1 已执行但候选未通过一致性验收；DirectML 尚未使用兼容运行时完成推理。

CUDA v1 浮点候选在 CPU 对照中也存在扩展参考样例的新增检测行。DirectML 转换候选的 CPU 回归一致，但本次取得的 DirectML ORT 1.24.4 无法提供 SDK 所需的 API 29，设备端推理未完成。


### 历史 v1 平台结果与运行时边界

- 原量化检测器在 macOS CoreML 与 CPU 的当前独立检测样例上保持一致，但 profile 显示主要计算仍由 CPU 执行。浮点转换候选在 CPU 上已改变部分框；同一候选切换 CoreML 又有额外差异。尺寸缓存没有解决这些几何差异。
- Windows x64 的 CPU Go/C++/Python 及独立检测/单行识别接口已实际检查。CUDA 也取得了真实 kernel 执行记录；原图仅加速检测器时，本轮完整 OCR 输出与 CPU 一致，但大部分检测计算仍在 CPU。原图全阶段 CUDA 和转换候选存在文字差异，不能推广为等价加速。
- 本次 Windows ORT 1.29 CUDA 二进制实际需要 CUDA 13 的 cuBLAS/cudart 和 cuDNN 9。隔离测试使用 cuBLAS 13.0.2.14、cudart 13.0.96、cuDNN 9.13.0.50；这描述已测试组合，不代表所有 ORT 构建的依赖相同。缺库时必须区分显式报错与已记录原因的 CPU 回退。
- 本次取得的官方 DirectML ORT 1.24.4 仅提供至 API 24，不能被当前 API 29 绑定初始化。需要兼容的运行时或独立兼容性工作，不能仅通过降低版本检查绕过 ABI 要求。
- 同一提交和输入的 CPU OCR 文字在 macOS/Windows 的复杂图片上也可能有差异；跨平台一致性需逐图核对。机器测量与逐图报告继续保存在仓库外。
