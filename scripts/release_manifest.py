#!/usr/bin/env python3
"""Create the versioned release manifest and SHA256SUMS; require all assets by default."""
from __future__ import annotations

import argparse
import json
from pathlib import Path

from build_sdk import copy, digest
from version import PYTHON_VERSION, ROOT, VERSION


def expected_assets():
    assets = {}
    for platform in ("linux-amd64", "linux-arm64"):
        assets[f"oneocr-{platform}.zip"] = ("app-full", platform)
    assets.update({
        "Model-NOTICE.txt": ("model-notice", ""),
        "LICENSE": ("license", ""),
        "oneocr-android.aar": ("android-full", "android"),
        "oneocr-android-core.aar": ("android-core", "android"),
        "oneocr-cjk-en.ocrpack": ("model", ""),
        f"oneocr_native-{PYTHON_VERSION}-py3-none-any.whl": ("python-wheel", ""),
        f"oneocr_native-{PYTHON_VERSION}.tar.gz": ("python-sdist", ""),
    })
    return assets


def create(directory: Path, partial: bool = False):
    expected = expected_assets()
    model = directory / "oneocr-cjk-en.ocrpack"
    copy(ROOT / "models/oneocr-cjk-en.ocrpack", model)
    copy(ROOT / "models/LICENSE", directory / "Model-NOTICE.txt")
    copy(ROOT / "LICENSE", directory / "LICENSE")
    missing = sorted(name for name in expected if not (directory / name).is_file())
    if missing and not partial:
        raise RuntimeError("incomplete release asset set: " + ", ".join(missing))
    assets = []
    for name, (kind, platform) in sorted(expected.items()):
        path = directory / name
        if path.is_file():
            assets.append({"name": name, "kind": kind, "platform": platform,
                           "bytes": path.stat().st_size, "sha256": digest(path)})
    manifest = directory / "release-manifest.json"
    manifest.write_text(json.dumps({"schema": "oneocr.release.v1", "version": VERSION, "assets": assets}, indent=2) + "\n", encoding="utf-8")
    lines = [f"{asset['sha256']}  {asset['name']}" for asset in assets]
    lines.append(f"{digest(manifest)}  {manifest.name}")
    (directory / "SHA256SUMS").write_text("\n".join(lines) + "\n", encoding="utf-8")
    return assets


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dist", type=Path, required=True)
    parser.add_argument("--partial", action="store_true", help="local testing only; never publish")
    args = parser.parse_args()
    assets = create(args.dist, args.partial)
    print(json.dumps({"assets": len(assets), "complete": len(assets) == len(expected_assets())}))
if __name__ == "__main__":
    main()
