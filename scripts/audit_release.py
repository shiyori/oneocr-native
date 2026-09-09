#!/usr/bin/env python3
"""Audit the exact public asset set, archive records, licenses and localized links."""
from __future__ import annotations
import argparse
import hashlib
import json
import re
import tarfile
import zipfile
from pathlib import Path, PurePosixPath
from release_manifest import expected_assets
from runtime_assets import digest
from version import ROOT, VERSION


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def audit_zip(path: Path, kind: str):
    with zipfile.ZipFile(path) as archive:
        names = archive.namelist()
        require(len(set(names)) == len(names), f"duplicate members: {path.name}")
        for name in names:
            parts = PurePosixPath(name).parts
            require(not name.startswith("/") and ".." not in parts and "\\" not in name, "unsafe archive path")
            require(not any(p in {"FORMAT.md", "ACCELERATION.md", ".git", "oneocr-extended.ocrpack"} for p in parts), f"private development file: {name}")
        records = [n for n in names if PurePosixPath(n).name == "SDK.json"]
        if kind in {"sdk-full", "sdk-core", "python-offline", "android-sdk"}:
            require(len(records) == 1, f"missing SDK record: {path.name}")
        for record_name in records:
            record = json.loads(archive.read(record_name))
            require(record["version"] == VERSION and record["schema"] == "oneocr.sdk.v1", "invalid SDK record")
            base = record_name.removesuffix("SDK.json")
            listed = set()
            for entry in record["files"]:
                name = base + entry["file"]
                require(name not in listed, "duplicate SDK record")
                listed.add(name)
                with archive.open(name) as stream:
                    sha = hashlib.file_digest(stream, "sha256").hexdigest()
                require(sha == entry["sha256"] and archive.getinfo(name).file_size == entry["bytes"], f"SDK checksum mismatch: {name}")
            require(listed == set(names) - {record_name}, "SDK record omits archive files")
        if kind in {"sdk-core", "android-core", "go-source", "python-wheel"}:
            require(not any(n.endswith(".ocrpack") or re.search(r"(?:^|/)libonnxruntime[^/]*\.(?:so|dylib)$|(?:^|/)onnxruntime.dll$", n) for n in names), f"core includes model/runtime: {path.name}")
        if kind in {"sdk-full", "android-full", "python-offline"}:
            require(any(n.endswith("oneocr-cjk-en.ocrpack") for n in names), f"default model missing: {path.name}")
        require(any("LICENSE" in n or "NOTICE" in n for n in names), f"license missing: {path.name}")
        if kind in {"sdk-full", "sdk-core", "go-source"}:
            require(any(n.endswith("go/internal/ort/onnxruntime_c_api.h") for n in names), "Go SDK omits C bridge headers")
            require(any(n.endswith("go/internal/ort/ONNXRUNTIME-LICENSE") for n in names), "Go SDK omits ORT header license")
        if kind == "python-wheel":
            metadata = archive.read(next(n for n in names if n.endswith(".dist-info/METADATA"))).decode()
            require(not re.search(r"^Requires-Dist: onnxruntime", metadata, re.M | re.I), "core wheel forces a runtime")
        if kind in {"android-full", "android-core"}:
            for abi in ("arm64-v8a", "x86_64"):
                for lib in ("liboneocr.so", "liboneocr_jni.so"):
                    require(f"jni/{abi}/{lib}" in names, "Android ABI library missing")
                if kind == "android-full":
                    require(f"jni/{abi}/libonnxruntime.so" in names, "Android runtime missing")


def audit_docs():
    count = 0
    files = [*ROOT.glob("README*.md"), *ROOT.glob("python/README*.md"), *ROOT.glob("docs/*/*.md"), ROOT / "sdk/README.md"]
    for path in files:
        content = path.read_text(encoding="utf-8")
        require("\ufffd" not in content, f"invalid UTF-8 text: {path}")
        for link in re.findall(r"\]\(([^)]+)\)", content):
            if "://" not in link and not link.startswith("#"):
                require((path.parent / link.split("#")[0]).exists(), f"broken local link: {path}: {link}")
                count += 1
        if path.parent.parent.name == "docs":
            for language in ("zh-CN", "en", "ja"):
                require(f"../{language}/{path.name}" in content, f"language switch changes topic: {path}")
    return count


def audit(directory: Path, partial: bool = False):
    expected = expected_assets()
    files = {p.name for p in directory.iterdir() if p.is_file() and not p.name.startswith(".")}
    require(not (files - set(expected) - {"release-manifest.json", "SHA256SUMS"}), "unexpected release asset")
    if not partial:
        require(set(expected) <= files, "incomplete release asset set")
    manifest = json.loads((directory / "release-manifest.json").read_text(encoding="utf-8"))
    require(manifest["version"] == VERSION and manifest["schema"] == "oneocr.release.v1", "release version mismatch")
    require({a["name"] for a in manifest["assets"]} == files - {"release-manifest.json", "SHA256SUMS"}, "release manifest asset mismatch")
    sums = dict(line.split("  ", 1)[::-1] for line in (directory / "SHA256SUMS").read_text().splitlines())
    for asset in manifest["assets"]:
        path = directory / asset["name"]
        require(digest(path) == asset["sha256"] == sums[path.name] and path.stat().st_size == asset["bytes"], "release checksum mismatch")
        require((asset["kind"], asset["platform"]) == expected[path.name], "asset kind/platform mismatch")
        if path.suffix in {".zip", ".aar", ".whl"}:
            audit_zip(path, asset["kind"])
        if asset["kind"] == "python-sdist":
            with tarfile.open(path) as archive:
                names = archive.getnames()
                require(not any("extended.ocrpack" in n or "/FORMAT.md" in n or "/ACCELERATION.md" in n for n in names), "private sdist file")
                require(any(n.endswith("/LICENSE") for n in names), "sdist license missing")
    require(digest(directory / "release-manifest.json") == sums["release-manifest.json"], "manifest checksum mismatch")
    return {"version":VERSION, "assets":len(manifest["assets"]), "links":audit_docs(), "passed":True, "complete":not partial}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dist", type=Path, required=True)
    parser.add_argument("--partial", action="store_true", help="local/build-job validation only")
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()
    report = audit(args.dist, args.partial)
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report))

if __name__ == "__main__":
    main()
