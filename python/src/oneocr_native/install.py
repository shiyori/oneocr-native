"""Release installation; intentionally uses only the Python standard library."""
from __future__ import annotations

import contextlib
import hashlib
import importlib
import importlib.metadata
import json
import os
import platform
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request
from pathlib import Path

from ._version import RELEASE_TAG, RUNTIME_VERSION
from .defaults import DEFAULT_MODEL, installation_home
from .errors import ModelFormatError

RELEASE_URL = f"https://github.com/shiyori/oneocr-native/releases/download/{RELEASE_TAG}/"


def _hash(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def _asset_stream(name: str, source: Path | None, offline: bool):
    if not name or Path(name).name != name or any(c in name for c in ("/", "\\")):
        raise ModelFormatError("invalid release asset name")
    if source is not None:
        return (source / name).open("rb")
    if offline:
        raise FileNotFoundError("offline resources are missing; pass --source RELEASE_DIRECTORY")
    return urllib.request.urlopen(RELEASE_URL + name, timeout=600)


def _read(name: str, source: Path | None, offline: bool) -> bytes:
    with _asset_stream(name, source, offline) as stream:
        data = stream.read(4 * 1024 * 1024 + 1)
    if len(data) > 4 * 1024 * 1024:
        raise ModelFormatError("release metadata exceeds 4 MiB")
    return data


def _model_asset(source: Path | None, offline: bool) -> dict:
    data = _read("release-manifest.json", source, offline)
    sums = _read("SHA256SUMS", source, offline).decode("utf-8")
    matches = [fields[0] for line in sums.splitlines()
               if len(fields := line.split()) == 2 and fields[1].lstrip("*") == "release-manifest.json"]
    if len(matches) != 1 or hashlib.sha256(data).hexdigest() != matches[0]:
        raise ModelFormatError("release manifest checksum mismatch")
    manifest = json.loads(data)
    if not isinstance(manifest, dict) or manifest.get("schema") != "oneocr.release.v1" or manifest.get("version") != RELEASE_TAG[1:]:
        raise ModelFormatError("release manifest version mismatch")
    assets = manifest.get("assets")
    if not isinstance(assets, list) or any(not isinstance(a, dict) for a in assets):
        raise ModelFormatError("invalid release asset list")
    selected = [a for a in assets if a.get("kind") == "model" and not a.get("platform")]
    if len(selected) != 1:
        raise ModelFormatError("release must contain exactly one default model")
    asset = selected[0]
    digest = asset.get("sha256", "")
    size = asset.get("bytes")
    name = asset.get("name")
    if (not isinstance(digest, str) or len(digest) != 64
            or any(c not in "0123456789abcdef" for c in digest)
            or type(size) is not int or not 0 < size <= 2 * 1024**3
            or not isinstance(name, str) or not name or name in {".", ".."}
            or any(c in name for c in ("/", "\\", "\x00"))):
        raise ModelFormatError("invalid model asset metadata")
    return asset


def _ensure_runtime(source: Path | None, offline: bool) -> None:
    installed = []
    for package in ("onnxruntime", "onnxruntime-gpu", "onnxruntime-directml"):
        try:
            installed.append((package, importlib.metadata.version(package)))
        except importlib.metadata.PackageNotFoundError:
            pass
    if installed:
        # Never replace or upgrade an existing CPU/GPU distribution.
        for name, version in installed:
            parts = version.split(".")
            if len(parts) < 2 or parts[0] != "1" or not parts[1].isdigit() or int(parts[1]) < 26:
                raise ModelFormatError(f"existing {name} {version} is incompatible; requires ORT 1.26 or newer 1.x")
        return
    command = [sys.executable, "-m", "pip", "install"]
    if offline or source is not None:
        if source is None:
            raise FileNotFoundError("offline runtime wheels are missing; use the complete Python bundle")
        wheels = source / "wheelhouse" / f"{sys.version_info.major}.{sys.version_info.minor}"
        if not wheels.is_dir():
            wheels = source / "wheelhouse"
        command += ["--no-index", "--find-links", str(wheels)]
    command += [f"onnxruntime=={RUNTIME_VERSION}"]
    subprocess.run(command, check=True)
    importlib.invalidate_caches()


@contextlib.contextmanager
def _lock(home: Path):
    # Same file and one-byte exclusive lock as the native installer.
    with (home / "install.lock").open("a+b") as stream:
        deadline = time.monotonic() + 120
        while True:
            try:
                if sys.platform == "win32":
                    import msvcrt
                    stream.seek(0)
                    msvcrt.locking(stream.fileno(), msvcrt.LK_NBLCK, 1)
                else:
                    import fcntl
                    fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
                break
            except OSError:
                if time.monotonic() >= deadline:
                    raise TimeoutError("another OneOCR installer is still running") from None
                time.sleep(0.1)
        try:
            yield
        finally:
            if sys.platform == "win32":
                stream.seek(0)
                msvcrt.locking(stream.fileno(), msvcrt.LK_UNLCK, 1)
            else:
                fcntl.flock(stream, fcntl.LOCK_UN)


def install(source: Path | None = None, *, offline: bool = False, home: Path | None = None) -> dict:
    home = (home or installation_home()).resolve()
    source = source.resolve() if source is not None else None
    home.mkdir(parents=True, exist_ok=True)
    with _lock(home):
        _ensure_runtime(source, offline)
        # Reuse the installed/default model before requesting any download.
        from .defaults import default_model_path
        model = None
        if source is not None:
            for candidate in (source / "models" / DEFAULT_MODEL, source / DEFAULT_MODEL):
                if candidate.is_file():
                    model = candidate
                    break
        if model is None:
            try:
                model = default_model_path(home)
            except (OSError, ModelFormatError):
                pass
        with tempfile.TemporaryDirectory(prefix=".prepare-", dir=home) as temporary:
            temporary = Path(temporary)
            if model is None:
                asset = _model_asset(source, offline)
                model = temporary / DEFAULT_MODEL
                with _asset_stream(asset["name"], source, offline) as stream, model.open("wb") as output:
                    count = 0
                    digest = hashlib.sha256()
                    while data := stream.read(1024 * 1024):
                        count += len(data)
                        if count > asset["bytes"]:
                            raise ModelFormatError("model download exceeds declared length")
                        digest.update(data)
                        output.write(data)
                    output.flush()
                    os.fsync(output.fileno())
                if count != asset["bytes"] or digest.hexdigest() != asset["sha256"]:
                    raise ModelFormatError("model download checksum mismatch")
            digest = _hash(model)
            destination = home / "models" / digest / DEFAULT_MODEL
            destination.parent.mkdir(parents=True, exist_ok=True)
            if not destination.is_file() or _hash(destination) != digest:
                pending = temporary / "model-install.ocrpack"
                shutil.copyfile(model, pending)
                os.replace(pending, destination)
            from .engine import EngineConfig, OneOcrEngine
            from .ocrpack import PackageSource
            with PackageSource(destination) as package:
                if package.info.get("profile") != "cjk-en":
                    raise ModelFormatError("install requires the default CJK/English model")
                provenance = package.info["bundle"]["source_sha256"]
            # Validate CPU operators and leave existing host sessions untouched.
            with OneOcrEngine(EngineConfig(destination)):
                pass
            configuration = {}
            path = home / "config.json"
            if path.is_file():
                if path.stat().st_size > 1024 * 1024:
                    raise ModelFormatError("installed configuration is too large")
                configuration = json.loads(path.read_text(encoding="utf-8"))
                if not isinstance(configuration, dict):
                    raise ModelFormatError("installed configuration must be an object")
            system = {"darwin": "darwin", "win32": "windows"}.get(sys.platform, "linux")
            arch = {"aarch64": "arm64", "arm64": "arm64", "amd64": "amd64", "x86_64": "amd64"}.get(platform.machine().lower())
            if arch is None:
                raise ModelFormatError("unsupported platform architecture")
            configuration.update(schema="oneocr.install.v2", platform=f"{system}-{arch}",
                                 model_path=str(destination), package_sha256=digest,
                                 model_sha256=provenance, profile="cjk-en")
            configuration.pop("bundle_dir", None)
            pending = temporary / "config.json"
            pending.write_text(json.dumps(configuration, indent=2) + "\n", encoding="utf-8")
            os.replace(pending, path)
    return configuration
