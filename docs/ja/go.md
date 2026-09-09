# Go

[简体中文](../zh-CN/go.md) | [English](../en/go.md) | [日本語](../ja/go.md)

[完全版デスクトップ SDK](installation.md) をダウンロードして展開します。Go 1.24 以上と C コンパイラーが必要です。自分のアプリの `main.go` に次のコードを保存します。

```go
package main

import (
    "context"
    "fmt"
    "github.com/shiyori/oneocr-native"
)

func main() {
    engine, err := oneocr.Open(oneocr.Config{})
    if err != nil { panic(err) }
    defer engine.Close()
    result, err := engine.Recognize(context.Background(), oneocr.FromFile("image.png"), oneocr.Options{})
    if err != nil { panic(err) }
    fmt.Println(result.Text)
}
```

**自分のアプリのディレクトリ**で次を実行します。既存の `go.mod` がある場合は `go mod init` を省略します。

```sh
go mod init example.com/ocr-app
/path/to/sdk/bin/oneocr install --go-project .
go run .
```

Windows では SDK の `bin\oneocr.exe` を使用します。準備コマンドが Go モジュールを OneOCR の管理ディレクトリへ保存し、プロジェクトを設定します。クローンや `replace` パスの手入力は不要です。完全版に含まれる依存関係だけで処理するには `--offline` を追加します。ホストの ORT バインディングのバージョンは変更しません。

コア版でも同じコマンドを使用でき、不足するモデルとランタイムを先に準備します。ソース依存関係を独自に管理するビルドシステム向けに、[Go ソースパッケージ](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-go-sdk-0.1.0-rc.1.zip) も用意しています。

## 入力と操作

```go
file := oneocr.FromFile("image.png")
encoded := oneocr.FromEncoded(pngOrJPEG)
img := oneocr.FromImage(goImage)
pixels := oneocr.FromPixels(oneocr.Pixels{
    Data: rgba, Width: width, Height: height, Stride: stride,
    Format: oneocr.RGBA,
})
```

すべての入力を `Recognize`、`Detect`、`RecognizeLine` で使用できます。同期呼び出し中は画像やバイト列を変更しないでください。JPEG のエンコード済み入力には EXIF の向きが適用されます。透明部分は白背景に合成され、ピクセル入力は sRGB と行ストライドに対応します。

```go
regions, err := engine.Detect(ctx, oneocr.FromFile("page.png"))
line, err := engine.RecognizeLine(ctx, oneocr.FromFile("line.png"), oneocr.Options{})
```

連続した処理では Engine を再利用します。同じインスタンスの操作は直列化され、Context の期限には待機時間も含まれます。`Close` は処理中の呼び出しを待ち、繰り返し呼び出せます。`Warmup(ctx)` は認識器を事前に読み込みます。終了後も `Diagnostics()` を取得できます。

`oneocr.Config` で `Threads`、`MaxSide`、`CharacterClasses` などを調整できます。既定設定ではモデルやランタイムのパスは不要です。[認識操作](stages.md)と[ランタイム再利用](runtime.md)も参照してください。

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)
