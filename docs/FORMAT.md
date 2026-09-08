# 已验证的 CBC OneModel 封装

本文描述本次附件对应版本。解析实现不调用或执行 DLL；DLL 只用于开发期静态分析。

| 输入 | SHA-256 |
| --- | --- |
| `oneocr.onemodel`（58,446,000 字节） | `f6cef38b839012cd824abf8b854ee9fa4f87d4c5265440661d66b23f4fab5155` |
| `oneocr.dll` | `43cd5e5813162d5046155a697b62dff0c45e953c61b4770d19ec69776848c2da` |

## 外层和资源索引

所有长度和偏移均为小端无符号 64 位整数。

```text
u64 encrypted_index_size
bytes encrypted_index[encrypted_index_size]
u64 body_size
bytes body[body_size]
```

附件的索引密文长度为 23,248，body 从文件偏移 23,264 开始，长度为 58,422,736。

外层索引使用调用方模型口令解密，得到：

```text
u64 encrypted_config_size
bytes encrypted_config[encrypted_config_size]
u64 resource_count
repeat resource_count:
    u64 encrypted_name_size
    bytes encrypted_name[encrypted_name_size]
    u64 offset_in_body
    u64 stored_size
    u8 encoding_flag
```

附件有 67 项，均为编码标志 1。索引名称、配置和资源正文分别解密；名称的解密不使用随机盐。输出文件使用数值索引命名，原始绝对路径只用于关联配置与资源。

## 加密记录

- 密钥为 `SHA256(key_material + salt)`，AES-256-CBC。
- IV 是 `Copyright @ OneO` 的 16 字节 ASCII。
- 带盐记录前 16 字节是 salt；不带盐记录使用空 salt。
- 外层索引的 key_material 是 OneOCR 包装器传入的 32 字节口令。
- 内层没有外部口令，salt 后的 16 字节（原始 payload 长度、payload 长度 + 24）作为 key_material，并从密文前移除。
- 使用 PKCS#7 去填充；补回前述两个长度字段后，完整明文信封为 `u64 payload_size, u64 total_size, u64 marker, payload`。
- `marker = 0x252B081A4A`，`total_size = payload_size + 24`。

代码同时检查填充、标记、全部长度、UTF-8 名称、索引唯一性和 body 的连续完整覆盖。未知编码拒绝处理。CBC 没有认证标签；这些检查及首次计算的 SHA-256 是结构校验和缓存身份标识，不是签名认证。

## 静态分析定位

以下为上述 DLL 中的 RVA（不含加载基址）：

| RVA | 用途 |
| --- | --- |
| `0x614730` | Crypto；设置 `ChainingModeCBC`，派生 SHA-256 密钥并调用 BCrypt |
| `0x6155c0` | 构造明文长度和 marker 信封 |
| `0x613880` | 外层文件读取、索引解密及 body 起点 |
| `0x613c70` | 配置、资源数量、名称和 offset/size/flag 索引解析 |
| `0x5a8dc0` | RelationRCNN 网格步长及回归缩放参数 |
| `0x5afa30` | Seglink 八邻接，任一方向连接概率达到 0.8 即连通 |
| `0x5b0500` | 四点框回归：网格中心为 `cell * stride + (stride - 1) / 2`，偏移乘 `8 * stride - 1` |

DLL 内嵌 `oneocr.proto` 与 `oneocr_interop.proto` 的 FileDescriptorProto。本包按核对过的字段号读取必需配置；不依赖 DLL 中的描述符文件，也不把未知字段解释为执行指令。缓存保留原始 protobuf，可用于后续补齐更多字段。

## 推理边界

该版本的全部 34 个 ONNX 都通过原生 ORT 校验和 CPU 冒烟推理。0 为检测器、1 为文字体系／方向分类器、2–10 为九类识别器、11–32 为拒识／校准网络、33 为 CJK 行布局网络；实际路径关联通过配置完成，不依赖这些数字作为公共 API。

检测器内部包含 RGB ImageNet mean/std，调用方传入 0–255 RGB；字符模型输入为 `[1,3,60,width]`，字典明确给出 `<blank>` 索引，不能假设 blank 为 0。CJK/Cyrillic 的帧步长为 8，其余附件识别器为 4。

连接通道顺序为左上、上、右上、左、右、左下、下、右下；对应反向通道为 `7-channel`。框回归使用以上 DLL 公式，不能把 cell 左上角当作中心，也不能用估计的锚点倍数替代。最终四边形拟合和 NMS 仍非原版完整实现。

运行链路目前使用检测器、辅助分类器和按需识别器。其他模型能够运行，但构造原版所需特征和调度仍有未恢复部分，不能因其能运行而声称已应用拒识或校准。后处理与完整误差记录见 README 和 validation 报告。
