"""Opt-in end-to-end regressions with the user's model; no bundled weights."""

import json
import os
from pathlib import Path

import pytest
from PIL import Image

from oneocr_native import EngineConfig, OneOcrEngine

pytestmark = pytest.mark.integration
MODEL = os.environ.get("ONEOCR_MODEL")
FIXTURES = os.environ.get("ONEOCR_FIXTURES")


@pytest.fixture(scope="module")
def engine():
    if not MODEL or not FIXTURES:
        pytest.skip("set ONEOCR_MODEL and ONEOCR_FIXTURES for native inference tests")
    with OneOcrEngine(EngineConfig(MODEL)) as value:
        yield value


# Hard gates use independently labelled simple cases, including one exact
# case per script. Difficult cases remain in evaluate.py's unfiltered report;
# their errors must not be normalized away to make these tests pass.
@pytest.mark.parametrize(
    "name",
    [
        "Latin",
        "CJK",
        "CJK_basic",
        "Cyrillic",
        "Arabic",
        "Devanagari",
        "Greek",
        "Hebrew",
        "Tamil",
        "Thai",
        "Arabic_numbers",
        "Hebrew_numbers",
        "Latin_rotate_90",
        "Latin_rotate_180",
        "Latin_rotate_270",
        "Latin_rotate_12",
        "low_contrast",
        "blank",
        "columns",
        "columns_rotate_90",
        "multilingual",
    ],
)
def test_native_image_recognition(engine, name):
    root = Path(FIXTURES)
    labels = {
        case["file"]: case["text"] for case in json.loads((root / "annotations.json").read_text())
    }
    result = engine.recognize(root / f"{name}.png")
    assert result.text == labels[f"{name}.png"]
    for line in result.lines:
        assert line.confidence is None
        assert len(line.quad) == 4
        for x, y in line.quad:
            assert 0 <= x <= result.width - 1
            assert 0 <= y <= result.height - 1


def test_rgba_and_path_inputs_agree(engine):
    root = Path(FIXTURES)
    with Image.open(root / "Latin.png") as image:
        assert engine.recognize(image.convert("RGBA")).text == "Hello World 123"


def test_model_inventory_and_all_submodel_inference(engine):
    assert set(engine.available_scripts) == {
        "Latin",
        "CJK",
        "Cyrillic",
        "Arabic",
        "Devanagari",
        "Greek",
        "Thai",
        "Hebrew",
        "Tamil",
    }
    resources = engine.prepared.manifest["resources"]
    models = [entry for entry in resources if entry["file"].endswith(".onnx")]
    assert len(resources) == 67 and len(models) == 34
    assert all(entry["validation"]["cpu_smoke_inference"] == "passed" for entry in models)


def test_independent_stages(engine):
    import cv2
    import numpy as np

    from oneocr_native.geometry import rectify

    root = Path(FIXTURES)
    detected = engine.detect(root / "Latin.png")
    assert len(detected.regions) == 1
    d = detected.regions[0]
    with Image.open(root / "Latin.png") as image:
        crop = rectify(np.asarray(image.convert("RGB")), np.asarray(d.quad), vertical=d.vertical)
    line = engine.recognize_line(Image.fromarray(crop), script="Latin")
    assert line.text == "Hello World 123"
    assert not line.rotated_180
    auto = engine.recognize_line(Image.fromarray(cv2.rotate(crop, cv2.ROTATE_180)))
    assert auto.text == line.text
    assert auto.rotated_180
    assert engine.recognize(root / "Latin.png").text == line.text
    assert detected.to_dict()["regions"][0]["score"] > 0
    with pytest.raises(ValueError, match="unavailable script"):
        engine.recognize_line(Image.fromarray(crop), script="invalid")
