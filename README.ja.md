# oneocr-native

[简体中文](README.md) · [English](README.en.md) · [日本語](README.ja.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/shiyori/oneocr-native.svg)](https://pkg.go.dev/github.com/shiyori/oneocr-native)
[![Go Report Card](https://goreportcard.com/badge/github.com/shiyori/oneocr-native)](https://goreportcard.com/report/github.com/shiyori/oneocr-native)
[![Stars](https://img.shields.io/github/stars/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/stargazers)
[![Downloads](https://img.shields.io/github/downloads/shiyori/oneocr-native/total)](https://github.com/shiyori/oneocr-native/releases)
[![Release](https://img.shields.io/github/v/release/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

オフライン OneOCR エンジン。ルート Go モジュール、Android AAR、C++ SDK、独立した Python SDK を提供します。共通の `.ocrpack` をリソース単位で読み込み、推論時に展開しません。Go/C++/Android は Python 不要です。Python は独自の OCR パイプラインで ORT を直接使用し、本プロジェクトの Go 動的ライブラリに依存しません。

交流・学習・研究を目的とした非公式の実験的実装です。この目的表記は MIT の用途を制限しません。第三者モデルの権利は変更されません。

## モデル

| Model | Scripts | Size |
|---|---|---:|
| [oneocr-cjk-en.ocrpack](models/oneocr-cjk-en.ocrpack) | 標準：中国語・日本語・韓国語・英語（CJK + Latin） | 30.46 MiB |
| [oneocr-extended.ocrpack](models/oneocr-extended.ocrpack) | キリル文字とアラビア文字を追加 | 40.07 MiB |

[Model origin / rights / checksums](models/README.md). Models are separate from SDKs and excluded from the root Go module ZIP by a nested data-only module.

## インストールと 4 SDK の利用

以下はソースからの導入手順です。`dist/` のローカル成果物は Git に含まれません。バージョンタグと Release は未公開です。ルートモジュール公開後は `go install github.com/shiyori/oneocr-native/cmd/oneocr@<tag>` を利用できます。依存関係の導入には通信が必要な場合がありますが、推論はオフラインです。

### Go / CLI

Go ≥1.24 + CGO + ONNX Runtime 1.29:

```bash
go install ./cmd/oneocr
oneocr install --model models/oneocr-cjk-en.ocrpack --runtime /path/to/libonnxruntime.dylib
oneocr recognize image.png
```

```go
import oneocr "github.com/shiyori/oneocr-native"

engine, err := oneocr.Open(oneocr.Config{ModelPath: "models/oneocr-cjk-en.ocrpack"})
if err != nil { return err }
defer engine.Close()
result, err := engine.RecognizeFile(ctx, "image.png", oneocr.Options{})
```

ORT: `ONEOCR_RUNTIME`, `Config.RuntimeLibrary`, SDK `lib/`. Local external module:

```bash
go mod edit -replace github.com/shiyori/oneocr-native=/path/to/oneocr-native
go get github.com/shiyori/oneocr-native
```

### Android

`dist/oneocr-android-0.1.0.aar` → `app/libs/`; `.ocrpack` → `app/src/main/assets/`:

```kotlin
android { defaultConfig { minSdk = 26 } }
dependencies { implementation(files("libs/oneocr-android-0.1.0.aar")) }
```

```java
// Run on a background thread. Reuse the engine.
try (dev.oneocr.OneOcr ocr = dev.oneocr.OneOcr.fromAsset(
        context, "oneocr-cjk-en.ocrpack", 2)) {
    String json = ocr.recognize(pngOrJpegBytes);
}
```

AAR: Java + JNI + Go native + ORT. `fromAsset` copies one file to private storage; no resource extraction.

### C++17

```cmake
find_package(OneOCR CONFIG REQUIRED)
add_executable(example main.cpp)
target_link_libraries(example PRIVATE OneOCR::oneocr)
```

`-DCMAKE_PREFIX_PATH=/path/to/oneocr-sdk-darwin-arm64`:

```cpp
#include <oneocr.hpp>
#include <iostream>
int main() {
    oneocr::Engine engine("oneocr-cjk-en.ocrpack");
    std::cout << engine.recognizeFile("image.png") << '\n';
}
```

### Python 3.11–3.13

```bash
python -m pip install ./python
# Or: python -m pip install dist/oneocr_native-0.1.0-py3-none-any.whl
oneocr-native recognize --model models/oneocr-cjk-en.ocrpack image.png
```

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine("models/oneocr-cjk-en.ocrpack") as engine:
    print(engine.recognize("image.png").text)  # PIL.Image.Image also accepted
```

## 検証状況と制限

| プラットフォーム | Go / C++ | 独立 Python | Android |
|---|---|---|---|
| macOS ARM64 | OCR・C++・外部 Go・移動した SDK で検証 | OCR と wheel 単独インストールを検証 | クロスビルド環境 |
| Android arm64-v8a / x86_64、API 26+ | Go native + JNI ビルド済み | Android Python 配布なし | AAR・利用側コンパイル・ELF 検査済み、端末 OCR 未検証 |
| Linux / Windows デスクトップ | ソース対応、適切な ORT/CGO が必要、今回は未実行 | ファイルロックと RTL を移植、今回は未実行 | 対象外 |

元の DLL と完全に同等ではありません。自動モードでは未収録の文字体系を警告付きでスキップし、明示指定ではエラーにします。結果はテキスト、行の四角形、文字体系、警告を含み、`confidence` と `words` は null です。領域結合と読み順は実験的で、手書きと自然な CJK 縦書きは未検証です。複雑な RTL 混在には曖昧性があります。Python は macOS で ICU、その他では Go と同様の移植可能な近似を使います。

Go/C++/Android は contrib ops を含む ONNX Runtime 1.29 CPU を使用します。デスクトップ SDK と AAR はランタイムを同梱し、Go ソース単体では別途必要です。Python は pip で ORT を導入します。ビルド成功は全端末での動作保証ではありません。

Android 互換性：ARM64 エミュレーターで ORT 1.29 の既定 KleidiAI 経路による SIGILL を確認しています。この経路を無効にした単体検出器は動作しましたが、SDK にはまだ回避設定を組み込んでおらず、Android OCR 全体の検証は完了していません。

## Documentation

[Go](docs/GO.md) · [Python](python/README.md) · [SDK](sdk/SDK.md) · [Build](docs/BUILD.md) · [OCRPACK](docs/PACK_FORMAT.md) · [Resources](docs/BUNDLE.md) · [OneModel](docs/FORMAT.md) · [Validation](validation/migration.json)

Source: [MIT](LICENSE). [Third-party notices](THIRD_PARTY_NOTICES.md).

## 高頻度呼び出しと実験的な高速化

Go/C/C++ は CPU を既定とし、CoreML・CUDA・DirectML、デバイス、処理段階ごとのフォールバック、文字種を明示的に設定できます。Engine を再利用し、`Warmup` で初期化してください。同時処理には少数の独立した Engine を使用します。認識結果の確率配列の追加コピーも削減しています。

オフラインの `oneocr-native adapt` ツールは、元モデルのハッシュを検証する実験用モデルを生成します。元モデルは保持されます。Provider の登録成功だけでは GPU 実行や速度向上を証明できません。CoreML はローカルで検証し、CUDA/DirectML は対応する実機での検証が必要です。[詳細](docs/ACCELERATION.md)。

Independent detection and cropped-line recognition: [API guide](docs/STAGES.md). Go, CLI, C/C++ and Python expose separate entry points.
