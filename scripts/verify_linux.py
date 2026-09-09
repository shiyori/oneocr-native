#!/usr/bin/env python3
"""Verify the relocated complete package and offline installation from official dependencies."""
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import tempfile
import zipfile
from pathlib import Path

from runtime_assets import PLATFORMS, current_platform, digest
from version import ROOT, RUNTIME_VERSION, VERSION

EXPECTED_TEXT = "你好世界 日本語テスト 한국어 123"


def environment(home: Path):
    env = {k: v for k, v in os.environ.items() if not k.startswith("ONEOCR_")}
    env.update(ONEOCR_HOME=str(home), HTTP_PROXY="http://127.0.0.1:9", HTTPS_PROXY="http://127.0.0.1:9",
               ALL_PROXY="http://127.0.0.1:9", NO_PROXY="", http_proxy="http://127.0.0.1:9",
               https_proxy="http://127.0.0.1:9", all_proxy="http://127.0.0.1:9", no_proxy="")
    return env


def command(executable, *args, work, env):
    result = subprocess.run([str(executable), *map(str, args)], cwd=work, env=env, text=True, capture_output=True, check=True)
    return result.stdout


def verify_table_result(result):
    labels = json.loads((ROOT / "scripts/testdata/medal-table-numbers.json").read_text())
    if len(result["lines"]) != 96:
        raise RuntimeError("table regression: missing recognized cells")
    cells = {}
    for line in result["lines"]:
        x = sum(p[0] for p in line["quad"]) / 4
        y = sum(p[1] for p in line["quad"]) / 4
        row = round((y - labels["row_origin"]) / labels["row_step"])
        column = min(range(6), key=lambda i: abs(x - labels["column_centers"][i]))
        if (row, column) in cells or not 0 <= row <= 15:
            raise RuntimeError("table regression: duplicate or misplaced cell")
        cells[row, column] = line["text"]
        if line["rotation_degrees"] not in (0, 90, 180, 270):
            raise RuntimeError("invalid crop rotation metadata")
    for row, values in enumerate(labels["rows"], 1):
        for column, expected in zip(labels["columns"], values):
            if cells.get((row, column)) != str(expected):
                raise RuntimeError(f"table regression at row {row}, column {column}: {cells.get((row, column))!r} != {expected}")


def verify(dist: Path, runtime_cache: Path):
    target = current_platform()
    if not target.startswith("linux-"):
        raise RuntimeError("Linux package verification requires its target Linux system")
    archive = dist / f"oneocr-{target}.zip"
    with tempfile.TemporaryDirectory(prefix="oneocr-linux-consumer-") as temporary:
        root = Path(temporary)
        with zipfile.ZipFile(archive) as source:
            for entry in source.infolist():
                if Path(entry.filename).is_absolute() or ".." in Path(entry.filename).parts or "\\" in entry.filename:
                    raise RuntimeError("unsafe package entry")
                path = Path(source.extract(entry, root))
                path.chmod((entry.external_attr >> 16) & 0o777)
        package = root / f"oneocr-{target}"
        moved = root / "complete package 含空格"
        package.rename(moved)
        work = root / "application"
        work.mkdir()
        image = work / "image.png"
        shutil.copy2(ROOT / "testdata/CJK.png", image)
        line_image = work / "line.png"
        shutil.copy2(ROOT / "scripts/testdata/cjk-line.png", line_image)
        env = environment(root / "unused-home")
        cli = moved / "bin/oneocr"
        for operation in ("recognize", "detect", "recognize-line"):
            input_image = line_image if operation == "recognize-line" else image
            result = json.loads(command(cli, operation, "--format", "json", input_image, work=work, env=env))
            if operation == "detect":
                if result["coordinate_space"] != "oriented_image" or any(
                    r["bbox"]["width"] <= 0 or r["bbox"]["height"] <= 0 for r in result["regions"]
                ):
                    raise RuntimeError("invalid detector JSON geometry")
                if not result["regions"]:
                    raise RuntimeError("complete package detector returned no regions")
            elif result["text"] != EXPECTED_TEXT:
                raise RuntimeError(f"complete package {operation} text mismatch: {result['text']!r}")
            if operation != "detect":
                if result["confidence_method"] != "ctc_token_geometric_mean" or not 0 < result["confidence"] <= 1:
                    raise RuntimeError("invalid recognition JSON confidence")
                if operation == "recognize" and (result["coordinate_space"] != "oriented_image" or not result["lines"] or any(
                    not 0 < line["confidence"] <= 1 or line["bbox"]["width"] <= 0 or line["detection_score"] <= 0
                    for line in result["lines"]
                )):
                    raise RuntimeError("incomplete recognition JSON line information")
        table = work / "table.png"
        shutil.copy2(ROOT / "testdata/paddleocr/720p-medal_table.png", table)
        verify_table_result(json.loads(command(cli, "recognize", "--format", "json", table, work=work, env=env)))
        # Code users may obtain resources separately. Exercise the same installer
        # with just the command, model metadata and unmodified official ORT archive.
        installer_dir = root / "installer"
        installer_dir.mkdir()
        installer = installer_dir / "oneocr"
        shutil.copy2(cli, installer)
        resources = root / "resources"
        resources.mkdir()
        for name in ("release-manifest.json", "SHA256SUMS", "oneocr-cjk-en.ocrpack"):
            shutil.copy2(dist / name, resources / name)
        upstream = f"onnxruntime-{PLATFORMS[target]}-{RUNTIME_VERSION}.tgz"
        shutil.copy2(runtime_cache / upstream, resources / upstream)
        env = environment(root / "installed-home")
        command(installer, "install", "--source", resources, "--offline", work=work, env=env)
        if command(installer, "recognize", image, work=work, env=env).strip() != EXPECTED_TEXT:
            raise RuntimeError("official-runtime installation OCR mismatch")
        command(installer, "install", "--offline", work=work, env=env)
    return {"platform": target, "version": VERSION, "archive_sha256": digest(archive),
            "relocated": True, "offline": True, "operations": ["recognize", "detect", "recognize-line"],
            "upstream_install": True, "repeat": True, "passed": True}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dist", type=Path, required=True)
    parser.add_argument("--runtime-cache", type=Path, required=True)
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()
    result = verify(args.dist.resolve(), args.runtime_cache.resolve())
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(result))


if __name__ == "__main__":
    main()
