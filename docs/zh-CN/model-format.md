# 模型包格式

[简体中文](../zh-CN/model-format.md) | [English](../en/model-format.md) | [日本語](../ja/model-format.md)

默认 `oneocr-cjk-en.ocrpack` 是单个 ONEOCRPK 容器。SDK 在创建模型会话前检查元数据与文件校验值，日常使用无需解压资源目录。

64 字节头部中，0–7 字节为 `ONEOCRPK`，8–15 字节为小端序版本和标志，16–23 字节为 JSON 索引长度，24–31 字节为数据区偏移，32–63 字节为索引 SHA-256。JSON 从第 64 字节开始；数据区及文件区间按 64 字节对齐，文件偏移相对于数据区起点。元数据记录 profile、源模型校验值、pipeline、文件尺寸和 SHA-256。读取时拒绝不安全名称、重复项、区间重叠和校验不匹配。

默认模型包包含检测器、分类器、中日韩英识别器及对应字典。识别保留原有 CPU 模型图和结果语义。

开发命令 `oneocr pack`、`inspect`、`unpack` 用于生成或检查模型包；`oneocr export` 保留旧目录 bundle 支持。Go 工具可调用 `Pack`、`ReadPackage`、`Unpack`。这些属于打包工具，独立于三个图像业务操作；应用直接使用默认安装模型即可。

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)
