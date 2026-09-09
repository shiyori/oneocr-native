"""Write localized galleries for the original PaddleOCR test images."""

from __future__ import annotations

import os
from pathlib import Path

from version import ROOT, VERSION

COPY = {
    "zh-CN": {
        "title": "PaddleOCR 原图测试结果",
        "intro": "使用公开发布的 OneOCR v{version}、默认中日韩英模型，对 `testdata/paddleocr` 中全部 {count} 张原图重新识别。输入保持原始尺寸，没有转换成 720p。",
        "filter": "仅在对照图中展示识别置信度 ≥ **{recognition:.2f}** 且检测分数 ≥ **{detection:.2f}** 的非空结果；JSON 保留全部识别输出。置信度为未校准模型分数。样本没有完整人工标注，下表数量不代表准确率。",
        "layout": "左图在原图上画框，右图在对应位置叠加蓝色识别文字和局部浅色底。两图保留完整画布，按 2 倍尺寸渲染；JSON 坐标对应原图。文字未经人工修正，点击图片可查看大图。",
        "columns": ["原图", "原始尺寸", "非空结果", "展示结果", "原始 JSON"],
        "boxes": "原图 + 文本框", "text": "原图 + 蓝色识别文字", "json": "JSON",
        "details": "原始尺寸：{width} × {height}；显示 **{displayed} / {recognized}** 条非空结果。",
        "source": "原图", "manifest": "生成清单", "reproduce": "复现方法",
        "license": "图片来源、固定提交与 SHA-256 见[来源清单](../../testdata/paddleocr/manifest.json)，图片保留上游 [Apache-2.0 许可](../../testdata/paddleocr/UPSTREAM_LICENSE)。",
        "home": "主 README", "root": "README.md",
    },
    "en": {
        "title": "PaddleOCR original-image test results",
        "intro": "All {count} original images in `testdata/paddleocr` were rerun with the public OneOCR v{version} release and the default Chinese/Japanese/Korean/English model. Inputs retain their original dimensions, without conversion to 720p.",
        "filter": "Paired images show nonempty results with recognition confidence ≥ **{recognition:.2f}** and detection score ≥ **{detection:.2f}**; JSON retains all recognition output. Confidence is an uncalibrated model score. These samples have no complete manual annotations, so the counts below are not accuracy measurements.",
        "layout": "The left image adds boxes to the original; the right overlays blue recognized text with local light backings. Both retain the full canvas and are rendered at 2× scale; JSON coordinates refer to the original image. Text is not manually corrected. Click an image to enlarge it.",
        "columns": ["Original", "Original dimensions", "Nonempty results", "Displayed results", "Raw JSON"],
        "boxes": "Original + text boxes", "text": "Original + blue recognized text", "json": "JSON",
        "details": "Original dimensions: {width} × {height}; displaying **{displayed} / {recognized}** nonempty results.",
        "source": "Original image", "manifest": "Generation manifest", "reproduce": "Reproduce",
        "license": "Sources, pinned commits and SHA-256 hashes are in the [source manifest](../../testdata/paddleocr/manifest.json). Image portions retain the upstream [Apache-2.0 license](../../testdata/paddleocr/UPSTREAM_LICENSE).",
        "home": "Main README", "root": "README.en.md",
    },
    "ja": {
        "title": "PaddleOCR 元画像のテスト結果",
        "intro": "`testdata/paddleocr` にある全 {count} 枚の元画像を、公開版 OneOCR v{version} と既定の中国語・日本語・韓国語・英語モデルで再認識しました。入力は元の解像度を維持し、720p には変換していません。",
        "filter": "比較画像には認識信頼度 ≥ **{recognition:.2f}**、検出スコア ≥ **{detection:.2f}** の空でない結果のみを表示し、JSON には全認識結果を保持します。信頼度は未校正のモデルスコアです。完全な手動正解データはないため、以下の件数は正解率ではありません。",
        "layout": "左は元画像に枠を追加し、右は同じ位置に青い認識文字と局所的な明るい背景を重ねています。両方とも全画面を保持して 2 倍で描画し、JSON の座標は元画像に対応します。文字の手動修正はありません。画像をクリックすると拡大できます。",
        "columns": ["元画像", "元の解像度", "空でない結果", "表示結果", "生の JSON"],
        "boxes": "元画像 + テキスト枠", "text": "元画像 + 青い認識文字", "json": "JSON",
        "details": "元の解像度：{width} × {height}；空でない結果 **{displayed} / {recognized}** 件を表示。",
        "source": "元画像", "manifest": "生成マニフェスト", "reproduce": "再現方法",
        "license": "出典、固定コミット、SHA-256 は[出典マニフェスト](../../testdata/paddleocr/manifest.json)を参照してください。画像部分には上流の [Apache-2.0 ライセンス](../../testdata/paddleocr/UPSTREAM_LICENSE)が適用されます。",
        "home": "メイン README", "root": "README.ja.md",
    },
}


def write_pages(metadata: dict, assets: Path) -> None:
    for language, c in COPY.items():
        page = ROOT / "docs" / language / "test-results.md"
        relative_assets = Path(os.path.relpath(assets, page.parent)).as_posix()
        entries = metadata["examples"]
        lines = [
            f"# {c['title']}", "",
            "[简体中文](../zh-CN/test-results.md) | [English](../en/test-results.md) | [日本語](../ja/test-results.md)", "",
            c["intro"].format(version=VERSION, count=len(entries)), "",
            c["filter"].format(recognition=metadata["recognition_confidence_min"], detection=metadata["detection_score_min"]), "",
            c["layout"], "",
            "| " + " | ".join(c["columns"]) + " |",
            "|---|---|---:|---:|---|",
        ]
        for e in entries:
            filename = Path(e["source"]).name
            width, height = e["source_size"]
            lines.append(f"| [{filename}](#sample-{e['name']}) | {width} × {height} | {e['recognized_lines']} | {e['displayed_lines']} | [{c['json']}]({relative_assets}/{e['result']}) |")
        for e in entries:
            filename = Path(e["source"]).name
            width, height = e["source_size"]
            boxes, text = (f"{relative_assets}/{e[key]}" for key in ("boxes", "text"))
            lines += [
                "", f'<a id="sample-{e["name"]}"></a>', "", f"## {filename}", "",
                c["details"].format(width=width, height=height, displayed=e["displayed_lines"], recognized=e["recognized_lines"]), "",
                f"[{c['source']}](../../{e['source']}) · [{c['json']}]({relative_assets}/{e['result']})", "",
                f"| {c['boxes']} | {c['text']} |", "|---|---|",
                f"| [![{filename} — {c['boxes']}]({boxes})]({boxes}) | [![{filename} — {c['text']}]({text})]({text}) |",
            ]
        lines += ["", c["license"], "",
                  f"[{c['manifest']}]({relative_assets}/manifest.json) · [{c['reproduce']}](../assets/ocr-results/README.md) · [{c['home']}](../../{c['root']})", ""]
        page.write_text("\n".join(lines), encoding="utf-8")
