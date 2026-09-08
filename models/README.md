# 模型 / Models / モデル

| 文件 | 文字体系 | 字节 | SHA-256 |
|---|---|---:|---|
| `oneocr-cjk-en.ocrpack` | CJK + Latin，中日韩英默认包 | 31,934,918 | `c0fe8fd89104d9c3e8154c03385232f8b6e4bfc92c1fdb59ff1ef5b93c4e1aa3` |
| `oneocr-extended.ocrpack` | CJK + Latin + Cyrillic + Arabic | 42,017,862 | `0a4aad220fd9770b5b3c6bd396f881024630a03585b80103b13bf42cf80ff923` |

来源：用户提供的 Microsoft OneOCR `oneocr.onemodel`，原始文件 SHA-256 为 `f6cef38b839012cd824abf8b854ee9fa4f87d4c5265440661d66b23f4fab5155`。仅选取现有运行管线依赖资源并重新封装，没有训练或改变权重。

这些第三方模型、字典和原配置不受仓库源码 MIT 许可重新授权。输入未附带独立的模型再分发许可；原始权利和适用条款仍归原权利人。见 [资源声明](LICENSE)。

These Microsoft OneOCR resources originate from the user-supplied file identified above. The source MIT license does not relicense model payloads; no separate redistribution grant accompanied the input. See [resource notice](LICENSE).

このモデルは上記のユーザー提供ファイルに由来します。ソースコードの MIT ライセンスはモデルを再許諾しません。入力には独立した再配布許諾が付属していません。[権利表記](LICENSE)を参照してください。

执行 `python3 scripts/verify_models.py`（仓库根目录）校验。文件由普通 Git 管理；本目录的嵌套 `go.mod` 使根模块的 Go 下载不包含模型。
