from __future__ import annotations

import argparse
import json
import sys
from dataclasses import asdict
from pathlib import Path

from .bundle import export_bundle
from .cache import prepare
from .config import PipelineConfig
from .container import ModelContainer
from .engine import OneOcrEngine
from .errors import OneOcrError


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Offline OCR for Chinese, Japanese, Korean and English")
    commands = parser.add_subparsers(dest="command", required=True)
    for command in ("inspect", "prepare", "recognize", "detect", "recognize-line", "export"):
        sub = commands.add_parser(command)
        sub.add_argument(
            "--model",
            type=Path,
            required=command in ("prepare", "export"),
            help="optional model path; uses the default model for recognition",
        )
        sub.add_argument("--output", type=Path, help="write result to a file instead of stdout")
        if command != "inspect":
            sub.add_argument("--cache-dir", type=Path)
        if command == "export":
            sub.add_argument("--directory", type=Path, required=True)
            sub.add_argument("--zip", type=Path, help="also create a complete portable ZIP bundle")
        if command in ("recognize", "detect", "recognize-line"):
            sub.add_argument("image", type=Path)
            sub.add_argument("--format", choices=("text", "json"), default="text")
            sub.add_argument(
                "--script", help="override automatic script detection, e.g. CJK or Latin"
            )
            sub.add_argument("--max-side", type=int, default=1600)
            sub.add_argument("--threads", type=int, default=2)
    args = parser.parse_args(argv)
    try:
        if hasattr(args, "model") and args.model is None:
            from .defaults import default_model_path

            args.model = default_model_path()
        if args.command == "inspect":
            from .ocrpack import MAGIC, PackageSource

            with args.model.open("rb") as stream:
                is_package = stream.read(8) == MAGIC
            if is_package:
                with PackageSource(args.model) as source:
                    result = source.info
            else:
                container = ModelContainer.load(args.model)
                config = PipelineConfig.parse(container.config)
                result = {
                    "source_sha256": container.source_hash,
                    "bytes": container.source_size,
                    "format": "OneModel AES-256-CBC",
                    "pipeline": asdict(config),
                    "resources": [
                        {"index": r.index, "name": r.name, "bytes": len(r.data), "sha256": r.digest}
                        for r in container.resources
                    ],
                }
            content = json.dumps(result, ensure_ascii=False, indent=2)
        elif args.command == "export":
            directory = export_bundle(
                args.model, args.directory, archive=args.zip, cache_dir=args.cache_dir
            )
            content = json.dumps(
                {"bundle": str(directory), "archive": str(args.zip) if args.zip else None},
                ensure_ascii=False,
            )
        elif args.command == "prepare":
            prepared = prepare(args.model, args.cache_dir)
            content = json.dumps(
                {
                    "cache": str(prepared.directory),
                    "manifest": str(prepared.directory / "manifest.json"),
                    "resources": len(prepared.manifest["resources"]),
                    "onnx_cpu_smoke_tests": sum(
                        "validation" in r for r in prepared.manifest["resources"]
                    ),
                },
                ensure_ascii=False,
                indent=2,
            )
        else:
            with OneOcrEngine(
                args.model, cache_dir=args.cache_dir, max_side=args.max_side, threads=args.threads
            ) as engine:
                if args.command == "detect":
                    result = engine.detect(args.image)
                elif args.command == "recognize-line":
                    result = engine.recognize_line(args.image, script=args.script)
                else:
                    result = engine.recognize(args.image, script=args.script)
            content = (
                json.dumps(result.to_dict(), ensure_ascii=False, indent=2)
                if args.format == "json" or args.command == "detect"
                else result.text
            )
        if args.output:
            args.output.write_text(content + "\n", encoding="utf-8")
        else:
            print(content)
        return 0
    except (OneOcrError, OSError, ValueError) as exc:
        print(f"oneocr-native: {exc}", file=sys.stderr)
        return 2
