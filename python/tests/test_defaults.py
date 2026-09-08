import hashlib
import json
import shutil
from pathlib import Path

import pytest

from oneocr_native import OneOcrEngine
from oneocr_native.defaults import DEFAULT_MODEL, default_model_path
from oneocr_native.errors import ModelFormatError


def test_default_never_selects_development_model(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    monkeypatch.delenv("ONEOCR_MODEL", raising=False)
    monkeypatch.setenv("ONEOCR_HOME", str(tmp_path / "home"))
    (tmp_path / "oneocr-extended.ocrpack").write_bytes(b"development-only")
    with pytest.raises(FileNotFoundError):
        default_model_path()


def test_installed_default_verifies_package_checksum(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    monkeypatch.delenv("ONEOCR_MODEL", raising=False)
    home = tmp_path / "installation"
    home.mkdir()
    monkeypatch.setenv("ONEOCR_HOME", str(home))
    model = home / DEFAULT_MODEL
    model.write_bytes(b"package")
    (home / "config.json").write_text(json.dumps({
        "schema": "oneocr.install.v2",
        "profile": "cjk-en",
        "model_path": str(model),
        "package_sha256": hashlib.sha256(b"package").hexdigest(),
    }))
    assert default_model_path() == model
    model.write_bytes(b"changed")
    with pytest.raises(ModelFormatError, match="checksum"):
        default_model_path()


def test_default_engine_recognizes_without_model_argument(tmp_path, monkeypatch):
    root = Path(__file__).resolve().parents[2]
    if not (root / "models" / DEFAULT_MODEL).is_file():
        pytest.skip("requires the repository's default model")
    monkeypatch.chdir(tmp_path)
    monkeypatch.delenv("ONEOCR_MODEL", raising=False)
    monkeypatch.setenv("ONEOCR_HOME", str(tmp_path / "home"))
    models = tmp_path / "models"
    models.mkdir()
    shutil.copyfile(root / "models" / DEFAULT_MODEL, models / DEFAULT_MODEL)
    with OneOcrEngine() as engine:
        assert engine.recognize(root / "testdata" / "CJK.png").text == (
            "你好世界 日本語テスト 한국어 123"
        )
