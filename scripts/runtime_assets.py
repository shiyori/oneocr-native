#!/usr/bin/env python3
"""Prepare pinned upstream ONNX Runtime files for a release build."""
from __future__ import annotations
import argparse
import hashlib
import json
import os
import platform
import shutil
import subprocess
import tarfile
import tempfile
import urllib.request
import zipfile
from pathlib import Path
from version import RUNTIME_VERSION

PLATFORMS = {"darwin-arm64": "osx-arm64", "linux-amd64": "linux-x64", "linux-arm64": "linux-aarch64", "windows-amd64": "win-x64"}

def digest(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()

def download(url: str, destination: Path, expected: str | None = None, algorithm: str = "sha256") -> Path:
    def actual(path):
        with path.open("rb") as stream:
            return hashlib.file_digest(stream, algorithm).hexdigest()
    if destination.is_file() and (expected is None or actual(destination) == expected):
        return destination
    destination.parent.mkdir(parents=True, exist_ok=True)
    temporary = destination.with_suffix(destination.suffix + ".download")
    with urllib.request.urlopen(url, timeout=600) as response, temporary.open("wb") as output:
        shutil.copyfileobj(response, output)
    if expected and actual(temporary) != expected:
        temporary.unlink()
        raise RuntimeError(f"upstream checksum mismatch: {url}")
    temporary.replace(destination)
    return destination

def current_platform() -> str:
    system = {"Darwin": "darwin", "Windows": "windows", "Linux": "linux"}[platform.system()]
    arch = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64", "amd64": "amd64"}[platform.machine().lower()]
    value = f"{system}-{arch}"
    if value not in PLATFORMS:
        raise RuntimeError(f"unsupported target: {value}")
    return value

def prepare(target: str, directory: Path, version: str = RUNTIME_VERSION) -> Path:
    directory = directory.resolve()
    directory.mkdir(parents=True, exist_ok=True)
    cache = directory / "downloads"
    if target == "android":
        name = f"onnxruntime-android-{version}.aar"
        url = f"https://repo.maven.apache.org/maven2/com/microsoft/onnxruntime/onnxruntime-android/{version}/{name}"
        checksum = urllib.request.urlopen(url + ".sha1", timeout=60).read().decode().split()[0]
        return download(url, cache / name, checksum, "sha1")
    name = f"onnxruntime-{PLATFORMS[target]}-{version}" + (".zip" if target.startswith("windows") else ".tgz")
    request = urllib.request.Request(f"https://api.github.com/repos/microsoft/onnxruntime/releases/tags/v{version}", headers={"User-Agent": "oneocr-release-builder"})
    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if token:
        request.add_header("Authorization", "Bearer " + token)
    metadata = json.load(urllib.request.urlopen(request, timeout=60))
    asset = next(a for a in metadata["assets"] if a["name"] == name)
    expected = (asset.get("digest") or "").removeprefix("sha256:") or None
    archive = download(asset["browser_download_url"], cache / name, expected)
    output = directory / f"runtime-{target}-{version}"
    if output.is_dir():
        shutil.rmtree(output)
    output.mkdir()
    with tempfile.TemporaryDirectory(dir=directory) as temporary:
        temporary = Path(temporary)
        if archive.suffix == ".zip":
            with zipfile.ZipFile(archive) as source:
                for entry in source.infolist():
                    if ".." in Path(entry.filename).parts or Path(entry.filename).is_absolute():
                        raise RuntimeError("unsafe upstream archive")
                source.extractall(temporary)
        else:
            with tarfile.open(archive) as source:
                source.extractall(temporary, filter="data")
        upstream = next(p for p in temporary.iterdir() if p.is_dir())
        library = {"darwin-arm64": "libonnxruntime.dylib", "windows-amd64": "onnxruntime.dll"}.get(target, "libonnxruntime.so")
        shutil.copy2(upstream / "lib" / library, output / library)
        for path in (upstream / "lib").iterdir():
            if path.is_file() and path.name != library and "onnxruntime" not in path.name and path.suffix in {".dll", ".dylib", ".so"}:
                shutil.copy2(path, output / path.name)
        for filename in ("LICENSE", "ThirdPartyNotices.txt"):
            shutil.copy2(upstream / filename, output / filename)
        # Windows ORT requires Microsoft's redistributable CRT. CI runs inside
        # the VS developer environment; include the licensed app-local DLL set.
        if target == "windows-amd64":
            roots = []
            if redist := os.environ.get("VCToolsRedistDir"):
                roots = list(Path(redist).glob("x64/Microsoft.VC*.CRT"))
            if not roots:
                base = Path(os.environ.get("ProgramFiles", "C:/Program Files")) / "Microsoft Visual Studio/2022"
                roots = list(base.glob("*/VC/Redist/MSVC/*/x64/Microsoft.VC*.CRT"))
            if not roots:
                raise RuntimeError("MSVC x64 redistributable CRT files are required for the complete Windows package")
            for path in sorted(roots)[-1].glob("*.dll"):
                shutil.copy2(path, output / path.name)
            (output / "MSVC-REDIST-NOTICE.txt").write_text("Microsoft Visual C++ Runtime files are redistributable under the Visual Studio license: https://visualstudio.microsoft.com/license-terms/\n", encoding="utf-8")
        files = [{"file": p.name, "bytes": p.stat().st_size, "sha256": digest(p)} for p in sorted(output.iterdir()) if p.is_file()]
        (output / "runtime.json").write_text(json.dumps({"schema": "oneocr.runtime.v1", "platform": target, "version": version, "library": library, "files": files}, indent=2) + "\n", encoding="utf-8")
    return output

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform", default=current_platform(), choices=[*PLATFORMS, "android"])
    parser.add_argument("--version", default=RUNTIME_VERSION)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    print(prepare(args.platform, args.output, args.version))
if __name__ == "__main__":
    main()
