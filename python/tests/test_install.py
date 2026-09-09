import hashlib
import json
import sys
from pathlib import Path

import pytest

from oneocr_native._version import RELEASE_TAG
from oneocr_native.errors import ModelFormatError
from oneocr_native.install import _asset_stream, _ensure_runtime, _model_asset


def metadata(directory: Path, manifest: dict):
    data = json.dumps(manifest).encode()
    (directory / "release-manifest.json").write_bytes(data)
    (directory / "SHA256SUMS").write_text(hashlib.sha256(data).hexdigest() + "  release-manifest.json\n")


def test_corrupt_release_and_wrong_version(tmp_path):
    manifest = {"schema":"oneocr.release.v1", "version":RELEASE_TAG[1:], "assets":[{
        "name":"model.ocrpack", "kind":"model", "bytes":3, "sha256":hashlib.sha256(b"abc").hexdigest()}]}
    metadata(tmp_path, manifest)
    assert _model_asset(tmp_path, True)["name"] == "model.ocrpack"
    (tmp_path / "release-manifest.json").write_bytes(b"{}")
    with pytest.raises(ModelFormatError, match="checksum"):
        _model_asset(tmp_path, True)
    manifest["version"] = "9.0.0"
    metadata(tmp_path, manifest)
    with pytest.raises(ModelFormatError, match="version"):
        _model_asset(tmp_path, True)


def test_offline_never_opens_network():
    with pytest.raises(FileNotFoundError, match="offline"):
        _asset_stream("model.ocrpack", None, True)
    with pytest.raises(ModelFormatError, match="name"):
        _asset_stream("../model.ocrpack", None, False)


@pytest.mark.parametrize("assets", [None, {}, [None], [{"kind":"model", "name":"x", "sha256":123, "bytes":"3"}]])
def test_malformed_asset_metadata(tmp_path, assets):
    metadata(tmp_path, {"schema":"oneocr.release.v1", "version":RELEASE_TAG[1:], "assets":assets})
    with pytest.raises(ModelFormatError):
        _model_asset(tmp_path, True)


def test_existing_gpu_runtime_is_kept(monkeypatch):
    import importlib.metadata
    import subprocess
    def version(name):
        if name == "onnxruntime-gpu":
            return "1.26.0"
        raise importlib.metadata.PackageNotFoundError(name)
    monkeypatch.setattr(importlib.metadata, "version", version)
    monkeypatch.setattr(subprocess, "run", lambda *a, **k: pytest.fail("replaced existing runtime"))
    _ensure_runtime(None, True)


def test_incompatible_host_is_not_upgraded(monkeypatch):
    import importlib.metadata
    import subprocess
    def version(name):
        if name == "onnxruntime":
            return "1.25.0"
        raise importlib.metadata.PackageNotFoundError(name)
    monkeypatch.setattr(importlib.metadata, "version", version)
    monkeypatch.setattr(subprocess, "run", lambda *a, **k: pytest.fail("upgraded existing runtime"))
    with pytest.raises(ModelFormatError, match="incompatible"):
        _ensure_runtime(None, False)


def test_installer_import_does_not_load_inference():
    import subprocess
    result = subprocess.run([sys.executable,"-c","import sys; import oneocr_native.install; assert 'onnxruntime' not in sys.modules; assert 'numpy' not in sys.modules; assert 'cv2' not in sys.modules"],capture_output=True,text=True,check=False)
    assert result.returncode == 0, result.stderr
