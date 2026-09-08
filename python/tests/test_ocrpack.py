"""Tracked packages, corruption checks, resource ownership and independent OCR."""

import hashlib
import json
import struct
from pathlib import Path

import pytest

from oneocr_native import OneOcrEngine
from oneocr_native.bidi import portable_visual_to_logical
from oneocr_native.errors import ModelFormatError, OneOcrError
from oneocr_native.ocrpack import PackageSource

ROOT = Path(__file__).resolve().parents[2]
MODELS = ROOT / "models"
FIXTURES = ROOT / "testdata"


@pytest.fixture(scope="module", params=["cjk-en", "extended"])
def engine(request):
    path = MODELS / f"oneocr-{request.param}.ocrpack"
    if not path.exists():
        pytest.skip("model data is distributed separately from Python source archives")
    with OneOcrEngine(path, threads=1) as value:
        yield value


def test_package_recognition(engine):
    labels = json.loads((FIXTURES / "annotations.json").read_text())
    for label in labels:
        result = engine.recognize(FIXTURES / label["file"])
        if label["file"].startswith("columns"):
            wanted = ["Hello World 123"] * 3
            if "Cyrillic" in engine.available_scripts:
                wanted += ["Привет мир 123"] * 3
            assert result.text == "\n".join(wanted)
            continue
        if label["file"] == "multilingual.png":
            wanted = [a["text"] for a in labels[:9] if a["script"] in engine.available_scripts]
            assert result.text == "\n".join(wanted)
            assert any("unavailable" in w for w in result.warnings)
            continue
        if label.get("script", "Latin") in engine.available_scripts:
            assert result.text == label["text"], label["file"]
        else:
            assert result.text == "", label["file"]
            assert any("unavailable" in w for w in result.warnings)
    with pytest.raises(ValueError):
        engine.recognize(FIXTURES / "Latin.png", script="Thai")


def test_readonly_and_close(tmp_path):
    source = MODELS / "oneocr-cjk-en.ocrpack"
    if not source.exists():
        pytest.skip("requires model package")
    model = tmp_path / "model.ocrpack"
    model.write_bytes(source.read_bytes())
    model.chmod(0o444)
    tmp_path.chmod(0o555)
    try:
        with OneOcrEngine.from_package(model) as value:
            descriptor = value.prepared._file
            assert not value.recognizers
            assert value.recognize(FIXTURES / "CJK.png").text == "你好世界 日本語テスト 한국어 123"
            assert set(value.recognizers) == {"CJK"}
        assert descriptor.closed and value.detector is None and value.classifier is None
        assert not value.recognizers
        value.close()
        with pytest.raises(OneOcrError, match="closed"):
            value.recognize(FIXTURES / "Latin.png")
        with pytest.raises(OneOcrError, match="closed"):
            value.__enter__()
        assert list(tmp_path.iterdir()) == [model]
    finally:
        tmp_path.chmod(0o755)
        model.chmod(0o644)


def _mutated_package(path, mutation):
    raw = bytearray(path.read_bytes())
    _, _, _, size, offset, _ = struct.unpack("<8sIIQQ32s", raw[:64])
    info = json.loads(raw[64 : 64 + size])
    mutation(info)
    index = json.dumps(info, separators=(",", ":")).encode()
    new_offset = (64 + len(index) + 63) & ~63
    header = struct.pack(
        "<8sIIQQ32s", b"ONEOCRPK", 1, 0, len(index), new_offset, hashlib.sha256(index).digest()
    )
    return header + index + bytes(new_offset - 64 - len(index)) + raw[offset:]


@pytest.mark.parametrize(
    "kind",
    [
        "magic",
        "version",
        "flags",
        "bounds",
        "index_hash",
        "resource_hash",
        "truncate",
        "trailing",
        "schema",
        "profile",
        "extent",
        "duplicate",
        "traversal",
        "missing",
        "pipeline",
        "resource_id",
        "negative_size",
    ],
)
def test_corrupt_package(tmp_path, kind):
    model = MODELS / "oneocr-cjk-en.ocrpack"
    if not model.exists():
        pytest.skip("requires model package")
    mutations = {
        "schema": lambda i: i.update(schema="unsupported"),
        "profile": lambda i: i.update(profile="full"),
        "extent": lambda i: i["files"][0].update(offset=64),
        "duplicate": lambda i: i["files"][1].update(file=i["files"][0]["file"]),
        "traversal": lambda i: i["files"][0].update(file="../escape"),
        "missing": lambda i: i["files"].pop(),
        "pipeline": lambda i: i["bundle"]["pipeline"].update(detector_path="missing.onnx"),
        "resource_id": lambda i: i["bundle"]["resources"][0].update(id=-1),
        "negative_size": lambda i: i["files"][0].update(bytes=-1),
    }
    if kind in mutations:
        raw = _mutated_package(model, mutations[kind])
    else:
        raw = bytearray(model.read_bytes())
        if kind == "truncate":
            del raw[-1:]
        elif kind == "trailing":
            raw += b"x"
        else:
            location = {
                "magic": 0,
                "version": 8,
                "flags": 12,
                "bounds": 16,
                "index_hash": 32,
                "resource_hash": -1,
            }[kind]
            raw[location] ^= 128
    target = tmp_path / "bad.ocrpack"
    target.write_bytes(raw)
    with pytest.raises(ModelFormatError):
        PackageSource(target)
    target.unlink()  # Windows also requires that failure closes the descriptor.


def test_session_failure_closes_source(monkeypatch):
    model = MODELS / "oneocr-cjk-en.ocrpack"
    if not model.exists():
        pytest.skip("requires model package")
    sources = []
    original = PackageSource.close

    def closed(source):
        sources.append(source)
        original(source)

    def fail(*_):
        raise RuntimeError("session failed")

    monkeypatch.setattr(PackageSource, "close", closed)
    monkeypatch.setattr("oneocr_native.engine.session", fail)
    with pytest.raises(RuntimeError):
        OneOcrEngine.from_package(model)
    assert sources and all(s._file.closed for s in sources)


@pytest.mark.parametrize(
    "visual,logical",
    [
        ("ملاعلاب ابحرم", "مرحبا بالعالم"),
        ("123.45 ריחמ", "מחיר 123.45"),
        ("USD 123.45 رعسلا", "السعر 123.45 USD"),
        ("بَ ا", "ا بَ"),
    ],
)
def test_portable_bidi(visual, logical):
    assert portable_visual_to_logical(visual) == logical
