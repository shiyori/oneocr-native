# ONEOCRPK v1：单文件模型容器

`.ocrpack` 是自定义、未压缩的资源容器，SDK 按索引直接读取模型字节交给 ONNX Runtime。它不需要解压目录，也不是合并权重后的单个 ONNX。原始 OneModel 仅作为打包输入；当前 SDK 的 `ModelPath` 接收 `.ocrpack`。

## 两种配置

| profile | 文字体系 | 模型／数据资源数 | 原始资源字节数 |
| --- | --- | ---: | ---: |
| `cjk-en`（默认） | CJK、Latin | 11 | 31,909,834（30.43 MiB） |
| `extended` | CJK、Latin、Cyrillic、Arabic | 20 | 41,986,365（40.04 MiB） |

两者都包含通用检测器和方向／文字体系分类器。资源数不计原始 `config.pb`、`pipeline.spec.json` 及索引。关联字表、字符映射、复合字符表和 rnn.info 保留；非目标识别器、当前未使用的拒识／校准／布局模型不进入默认分发包。完整版原始资源仍可通过 `oneocr export` 导出为标准目录。

## 二进制布局

文件最大 2 GiB，整数均为小端，无压缩或加密。定长文件头：

| 偏移 | 长度 | 字段 |
| ---: | ---: | --- |
| 0 | 8 | ASCII 魔数 `ONEOCRPK` |
| 8 | 4 | uint32 版本，固定 1 |
| 12 | 4 | uint32 flags，固定 0，未知标志拒绝 |
| 16 | 8 | uint64 JSON 索引长度，1 字节至 16 MiB |
| 24 | 8 | uint64 资源数据区绝对偏移 |
| 32 | 32 | 索引原始 UTF-8 字节的 SHA-256 |

索引从文件偏移 64 开始。数据区偏移为 `align64(64 + index_length)`，间隙填零。每项数据的相对偏移也按 64 字节对齐，资源间隙填零；最后一项数据结束即文件结束，不追加尾部数据。

JSON 索引结构：

```json
{
  "schema": "oneocr.pack.v1",
  "profile": "cjk-en",
  "bundle": { "schema": "oneocr.bundle.v1", "...": "完整的现有 bundle manifest" },
  "files": [
    {"file": "config.pb", "bytes": 11902, "sha256": "64位十六进制摘要", "offset": 0}
  ]
}
```

示例中的省略字段须由实际 manifest 替代。`bundle` 的字段定义见 `BUNDLE.md`，其 `pipeline` 只引用选中 profile 的依赖；原始配置保留全部来源信息，不用它覆盖已裁剪或自定义的 pipeline。

`files` 包含所有 `bundle.resources`，加上原始配置和 `pipeline.spec.json`。按资源路径的字节字典序排序，`offset` 相对数据区。名称唯一且禁止大小写折叠后冲突；路径不得绝对化、包含 `..`、反斜杠、盘符或 NUL。资源偏移必须等于上一项结束位置向上对齐到 64 的值，不能重叠或留下未声明数据。资源的大小／摘要须与内嵌 manifest 一致。

默认命名仍为 `models/recognition/cjk_printed.onnx`、`data/recognition/cjk_printed/alphabet.txt` 等标准语义路径；不依赖资源 ID 做运行时路由。哈希用于损坏检测，模型权重保持原样。

## 打包与自定义

```bash
oneocr pack --model /absolute/path/oneocr.onemodel --profile cjk-en --output oneocr-cjk-en.ocrpack
oneocr pack --bundle /absolute/path/standard-bundle --profile extended --output oneocr-extended.ocrpack
oneocr inspect --model oneocr-cjk-en.ocrpack
oneocr unpack --model oneocr-cjk-en.ocrpack --directory editable-bundle
# 修改 editable-bundle 中资源／pipeline，更新相应 bytes、sha256 与 ONNX 接口描述
oneocr pack --bundle editable-bundle --profile cjk-en --output customized.ocrpack
```

输出文件／目录必须不存在。Go API 为 `Pack(PackOptions{...})`、`ReadPackage(filename)`、`Unpack(filename, directory)`。相同输入和配置会生成相同字节的包，解包后原样重新打包也一致。

## 加载与生命周期

`Open(Config{ModelPath: path})` 保持一个只读文件描述符，启动时用有限大小缓冲区校验所有数据，加载模型时只读取当前资源，并再次校验其摘要。识别器按需创建，模型原始字节在 ORT session 创建后可回收；不会把完整包常驻为一个大字节数组。`Close` 等待识别结束，销毁 session 并关闭描述符。使用期间不要原地改写模型文件；更新部署使用另一个文件或原子替换。

旧目录入口 `Config.BundleDir` 保持兼容，两者只能指定一个。自动分类到包外脚本时跳过对应行并在 `warnings` 中记录数量；显式指定包外 `Options.Script` 返回错误。CJK／Latin 路由、CTC、阅读顺序及置信度／词框为空的既有行为保留。

SDK 的二进制、Android AAR 与模型文件分开分发。SDK 可以包含平台 ORT，容器本身不含任何平台代码。
