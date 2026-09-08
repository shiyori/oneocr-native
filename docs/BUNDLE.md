# 完整 OneOCR 资源包：oneocr.bundle.v1

默认应用集成请使用 [单文件模型包](PACK_FORMAT.md) 和 [SDK](../sdk/SDK.md)；本页是完整资源的开发者编辑格式。

资源与平台分开：同一个 bundle 可被 Go、Python 或自行编写的 C++/Android ONNX 管线使用，平台只替换 ONNX Runtime 动态库。Go 导出器无需 Python、ORT 或 oneocr.dll；它从 OneModel 读取全部资源，不仅提取当前 OCR 路径使用的模型。

```bash
oneocr export --model /path/oneocr.onemodel --directory /path/oneocr-bundle --zip /path/oneocr-bundle.zip
```

Python 同样支持 `oneocr-native export --model ... --directory ... --zip ...`，并支持 `OneOcrEngine.from_bundle("/path/oneocr-bundle")`。

## 文件与清单

```text
oneocr-bundle/
  bundle.json          # v1 清单、原始资源名、hash、大小、ONNX 接口、可编辑 pipeline
  pipeline.spec.json   # 参考预处理／解码约定，说明用途，不是可执行配置
  config.pb            # 原始 protobuf 配置，字节原样保留
  models/detection/universal.onnx
  models/classification/script_orientation.onnx
  models/recognition/latin_printed_v2.onnx
  models/recognition/cjk_printed.onnx
  models/rejection/latin_printed_v2_dummy.onnx
  models/confidence/latin_printed_v2_dummy.onnx
  models/layout/cjk.onnx
  data/recognition/latin_printed_v2/alphabet.txt
  data/recognition/latin_printed_v2/character_mapping.txt
  data/recognition/latin_printed_v2/composite_characters.txt
  data/recognition/latin_printed_v2/rnn.info
  data/detection/universal/checkbox_calibration.txt
  data/classification/handwriting_calibration.txt
  data/enums/enums.edge.prototxt
  ...                  # 其他脚本按相同规则命名
```

统一命名规范：目录和文件名采用小写英文与 `snake_case`，按“资源类型／用途／模型配置”分层。识别器保留 `mixed`、`printed`、`v2` 等模型配置差异；拒识与校准资源保留原配置的 `dummy` 标识，避免不同模型被合并。公共文件名不包含资源序号、训练 checkpoint、时间戳或机器构建路径；例如 `CJKPrinted` 对应 `cjk_printed`。字表为 `alphabet.txt`，字符映射为 `character_mapping.txt`，复合字符表为 `composite_characters.txt`；`rnn.info` 保留其原有格式名。

`resources[].id` 仅作为来源索引，`original_name` 保留原始名称，所有实际加载路径均从 manifest 中读取。Go/Python 导出器同步采用此规范；若两个原资源归一化为同一路径，导出报错，不覆盖文件。未识别的资源类型使用安全化的原始文件名；内部 Python 缓存仍使用数字编号，与公开 bundle 分开。

旧数字命名的 v1 bundle 仍能读取。已有导出目录会保留，不自动改写自定义文件；重新导出时请选择新目录。通过 `oneocr export` 可生成完整标准命名资源目录。

当前模型来源 SHA-256：`f6cef38b839012cd824abf8b854ee9fa4f87d4c5265440661d66b23f4fab5155`。
67 个资源合计 **58,419,536 字节**；其中 34 个 ONNX 合计 56,938,990 字节，33 个数据资源合计 1,480,546 字节。`config.pb` 另为 11,902 字节。Go 与 Python 导出资源的每个字节、原始配置及规范化 JSON 清单已逐项比对一致。

| ID | 用途 | 当前 SDK 是否使用 |
| --- | --- | --- |
| 0 | RelationRCNN / Seglink 通用检测器 | 是 |
| 1 | 文字体系、手写体与 180° 翻转分类 | 使用 script、flip 输出 |
| 2–10 | Arabic、CJK、Cyrillic、Devanagari、Greek、Hebrew、Latin、Tamil、Thai 识别器 | 按需 |
| 11–21 | 11 个拒识模型 | 导出保留；原版输入特征生成尚未复现 |
| 22–32 | 11 个置信度校准模型 | 导出保留；不据此伪造置信度 |
| 33 | CJK 行布局模型 | 导出保留；当前阅读顺序另行实现 |
| 34–35 | checkbox、手写体校准数据 | 导出保留 |
| 36–65 | 九类识别器的先验、字表、物理映射、复合字符映射 | 字表和复合映射用于解码，其余保留 |
| 66 | enums.edge.prototxt | 导出保留 |

以上 ID 只针对这份模型。开发者应读取 `pipeline` 和 `resources[].original_name`，不要跨版本硬编码序号；容器中的 Windows 构建路径只是元数据，不是本地文件路径。

`bundle.json` 包含：

- `schema`：固定 `oneocr.bundle.v1`；`source_sha256` 表示原 OneModel 的来源，不代表自定义权重集合的新摘要。
- `runtime`：ORT 1.29.0、CPU provider、需要 contrib ops。
- `config`：原始配置的相对路径、字节数与 SHA-256。
- `resources[]`：唯一 ID、相对路径、原名、类型、字节数、SHA-256；模型还含 `interface.inputs/outputs` 的名称、dtype、shape，以及 `opsets` 和 `operators`。
- `pipeline`：检测／分类模型相对路径、脚本模型及字表／映射／先验路径、帧步长、检测阈值。

shape 中整数为固定维，字符串为动态维名，`null` 为未命名动态维。初始权重已从模型接口输入列表中排除。路径使用 `/`，不允许绝对路径、`..`、Windows 盘符或向目录外跳转的符号链接。SDK 每次 Open 校验所有资源大小和 hash；哈希用于完整性校验，不是发行方数字签名。

## 自定义方式

先复制一个 bundle 再修改。可直接调整 `bundle.json.pipeline` 的分段阈值、P2/P3/P4 行阈值、脚本模型列表及路径。可以只保留少数脚本以减少按需加载范围；若删除资源文件，同时删去对应 `resources` 条目和所有 pipeline 引用。

替换权重、字表或映射时，同步更新对应 `resources[].bytes` 和 `sha256`，ONNX 的 `interface` 也要与新模型一致。Go/Python SDK 使用 `bundle.json.pipeline`，不重新解析 `config.pb` 覆盖自定义值；原始配置用于来源留存。`pipeline.spec.json` 是说明文件，修改它不会改变 SDK 算法。需要调整链接规则、归一化高度、解码器、NMS 或版面分组时，请修改 Go 源码相应领域文件，或自行实现管线。

兼容替换必须保持 SDK 所需张量名称和语义；修改 JSON 不能把任意检测器／识别器变成兼容模型。字表类别数必须与识别器输出一致，明确读取 `<blank>` 的索引，不能假定索引 0 是 CTC blank。

## 自写 ONNX 管线的关键约定

1. 读取图片并应用 EXIF 方向，RGB 白底。检测时只缩小不放大，默认最长边 1600，右侧／底部补白到 32 的倍数。
2. 检测输入 `data` 为 float32 NCHW、数值 0–255；归一化已内嵌图中，勿再除以 255。`im_info` 为 `[1,3]`，内容是缩放后的高、宽和 1。
3. 读取横／纵两组 P2/P3/P4 的 score、bbox、link。stride=`2^level`，中心偏移 `(stride-1)/2`，回归比例 `8*stride-1`。八邻接按 NW/N/NE/W/E/SW/S/SE；任一方向链接分数 ≥0.8 即连通，反向通道为 `7-channel`。实际阈值以 pipeline 为准。
4. 当前独立后处理按连通区域拟合最小矩形，quad IoU >0.2 做 NMS，映射回原图。透视裁出行图；竖组且高大于宽时逆时针转 90°。
5. 行归一化到高 60，宽按比例取整，左右各补 16，再按帧步长补齐；float32 RGB NCHW、范围 0–1。脚本分类的对齐步长为 4；`flip_score <0` 时旋转 180°。
6. 识别输入 `data` 和 int32 `seq_lengths=[padded_width/pixels_per_frame]`，CJK/Cyrillic 帧步长 8，其余为 4；输出 `logsoftmax` 为 `[T,1,classes]`。
7. greedy CTC 合并相邻重复并去 blank，展开复合字符映射，处理视觉到逻辑 RTL 顺序及 NFC；最后按页面方向与列间空白排序。

细节以 [Go 实现](../) 和每个模型实际接口为准。拒识／置信度模型虽然可以在 ORT 中执行，其原版输入特征不等同于识别 logits；资源齐全不表示完整 DLL 行为已经还原。
