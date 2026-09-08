# 默认模型

公开 SDK 使用 `oneocr-cjk-en.ocrpack`，支持中文、日文、韩文、英文和数字。

- 文件大小：31,934,918 字节。
- SHA-256：`c0fe8fd89104d9c3e8154c03385232f8b6e4bfc92c1fdb59ff1ef5b93c4e1aa3`。
- 桌面与 Python：放入工作目录的 `models/`。
- Android：放入 `app/src/main/assets/`。

资源来自 Microsoft OneOCR，按现有资源重新封装，没有训练或改变权重。模型、字典和原配置不受仓库源码 AGPL-3.0-only 许可重新授权；原始权利与适用条款仍归原权利人，见 [资源声明](LICENSE)。

本目录由普通 Git 管理；嵌套的 `go.mod` 使根 Go 模块下载不包含模型。其余模型文件仅作为仓库开发备用，不属于公开 SDK 使用入口或发行内容。
