# Go

[简体中文](../zh-CN/go.md) | [English](../en/go.md) | [日本語](../ja/go.md)

Go 1.24 以上と C コンパイラーが必要です。自分のプロジェクトに Go モジュールを追加します。デスクトップ SDK のダウンロードは不要です。

```sh
go mod init example.com/ocr-app
go get github.com/shiyori/oneocr-native@v0.1.1
```

既に `go.mod` がある場合は `go mod init` を省略してください。

次のコードを自分の `main.go` に保存します。`Install` は不足しているモデルと依存関係を準備します。準備済みの環境では直接 `Open` を呼べます。

```go
package main

import (
    "context"
    "fmt"
    "github.com/shiyori/oneocr-native"
)

func main() {
    if _, err := oneocr.Install(oneocr.InstallOptions{}); err != nil { panic(err) }
    engine, err := oneocr.Open(oneocr.Config{})
    if err != nil { panic(err) }
    defer engine.Close()
    result, err := engine.Recognize(context.Background(), oneocr.FromFile("image.png"), oneocr.Options{})
    if err != nil { panic(err) }
    fmt.Println(result.Text)
}
```

```sh
go run .
```

## コマンドのインストール

Go でコマンドをインストールした後、任意のディレクトリでリソースを準備して画像を認識できます。

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.1
oneocr install
oneocr recognize image.png
```

Go の実行ファイル用ディレクトリ（通常は `GOPATH/bin`）を `PATH` に追加してください。どちらの方法も互換 runtime を自動検出して再利用し、不足分を必要に応じて取得します。通常の呼び出しに runtime パスは不要で、ホストの ORT Go バインディングのバージョンも変更しません。

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
