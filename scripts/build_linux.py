#!/usr/bin/env python3
"""Build one ready-to-run Linux package per architecture."""
from __future__ import annotations

import argparse
import json
import tempfile
from pathlib import Path

from build_sdk import archive, copy, copy_docs, licenses, run
from runtime_assets import current_platform, digest, prepare
from version import ROOT, RUNTIME_VERSION, VERSION


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT / "dist/release")
    parser.add_argument("--runtime-directory", type=Path)
    args = parser.parse_args()
    target = current_platform()
    if not target.startswith("linux-"):
        raise RuntimeError("Build this package on its target Linux architecture")
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    runtime = args.runtime_directory or prepare(target, output / ".upstream")
    with tempfile.TemporaryDirectory(prefix=".linux-package-", dir=output) as temporary:
        stage = Path(temporary) / f"oneocr-{target}"
        (stage / "bin").mkdir(parents=True)
        run("go", "build", "-trimpath", "-o", stage / "bin/oneocr", "./cmd/oneocr", cwd=ROOT)
        for path in runtime.iterdir():
            if path.is_file():
                copy(path, stage / "lib" / path.name)
        copy(ROOT / "models/oneocr-cjk-en.ocrpack", stage / "models/oneocr-cjk-en.ocrpack")
        copy(ROOT / "models/LICENSE", stage / "models/LICENSE")
        copy_docs(stage)
        licenses(stage / "licenses", runtime)
        files = [{"file": p.relative_to(stage).as_posix(), "bytes": p.stat().st_size, "sha256": digest(p)}
                 for p in sorted(stage.rglob("*")) if p.is_file()]
        (stage / "PACKAGE.json").write_text(json.dumps({
            "schema": "oneocr.distribution.v1", "version": VERSION, "platform": target,
            "onnxruntime": RUNTIME_VERSION, "runtime_included": True, "models_included": True,
            "entrypoint": "bin/oneocr", "files": files,
        }, indent=2) + "\n", encoding="utf-8")
        destination = output / f"oneocr-{target}.zip"
        archive(stage, destination)
    print(json.dumps({"file": str(destination), "bytes": destination.stat().st_size, "sha256": digest(destination)}))


if __name__ == "__main__":
    main()
