import math
import os
from pathlib import Path

import numpy as np
import pytest
from PIL import Image

from oneocr_native import EngineConfig, OneOcrEngine
from oneocr_native.geometry import rectify
from oneocr_native.recognition import Recognition
from oneocr_native.routing import (
    compact_quad,
    numeric_consensus,
    orientation_turns,
    page_orientation,
)


def scored(text, probability):
    return Recognition(text, math.log(probability), 1)


@pytest.mark.parametrize(
    "a,b,expected",
    [
        (scored("8", 0.95), scored("8", 0.60), True),
        (scored("6", 0.99), scored("9", 0.99), False),
        (scored("8", 0.70), scored("8", 0.99), False),
        (scored("8", 0.99), scored("8", 0.30), False),
        (scored("*", 0.99), scored("*", 0.99), False),
        (Recognition(), Recognition(), False),
        (scored("1234", 0.99), scored("1234", 0.99), False),
    ],
)
def test_numeric_consensus(a, b, expected):
    assert numeric_consensus(a, b) == expected


def test_page_orientation_requires_agreement():
    q = np.array([[0, 0], [10, 0], [10, 12], [0, 12]], np.float32)
    for angle, turns in [(0, 0), (math.pi / 2, 1), (math.pi, 2), (-math.pi / 2, 3)]:
        prior = page_orientation([(angle, 10)])
        assert prior is not None and orientation_turns(q, prior) == turns
    assert page_orientation([(0, 3), (math.pi, 3)]) is None
    assert page_orientation([]) is None
    assert compact_quad(q)
    assert not compact_quad(np.array([[0, 0], [50, 0], [50, 10], [0, 10]], np.float32))


def cell(quad, rotation=0):
    x, y = np.asarray(quad).mean(axis=0)
    if rotation == 90:
        x, y = y, 719 - x
    elif rotation == 180:
        x, y = 1279 - x, 719 - y
    elif rotation == 270:
        x, y = 1279 - y, x
    centers = [392.63, 537.37, 687.90, 741.70, 795.42, 867.48]
    return round((y - 200.16) / 21.33), min(range(6), key=lambda c: abs(x - centers[c]))


@pytest.mark.integration
def test_table_numerals_and_rotated_rank():
    model = os.environ.get("ONEOCR_MODEL")
    fixtures = os.environ.get("ONEOCR_FIXTURES")
    if not model or not fixtures:
        pytest.skip("set ONEOCR_MODEL and ONEOCR_FIXTURES")
    image = Image.open(Path(fixtures) / "paddleocr/720p-medal_table.png").convert("RGB")
    values = [
        [1, 48, 22, 30, 100],
        [2, 36, 39, 37, 112],
        [3, 24, 13, 23, 60],
        [4, 19, 13, 19, 51],
        [5, 16, 11, 14, 41],
        [6, 14, 15, 17, 46],
        [7, 13, 11, 8, 32],
        [8, 9, 8, 8, 25],
        [9, 8, 9, 10, 27],
        [10, 7, 16, 20, 43],
        [11, 7, 5, 4, 16],
        [12, 7, 4, 11, 22],
        [13, 6, 4, 6, 16],
        [14, 5, 11, 3, 19],
        [15, 5, 4, 2, 11],
    ]
    with OneOcrEngine(EngineConfig(model, threads=1)) as engine:
        result = engine.recognize(image)
        assert len(result.lines) == 96
        cells = {cell(line.quad): line for line in result.lines}
        for r, row in enumerate(values, 1):
            for c, value in zip([0, 2, 3, 4, 5], row):
                assert cells[r, c].text == str(value), (r, c, cells[r, c].text, value)
        assert (
            sum(line.confidence >= 0.70 and line.detection_score >= 0.70 for line in result.lines)
            == 96
        )
        for angle, transform in [
            (90, Image.Transpose.ROTATE_270),
            (180, Image.Transpose.ROTATE_180),
            (270, Image.Transpose.ROTATE_90),
        ]:
            result = engine.recognize(image.transpose(transform))
            cells_rotated = {cell(line.quad, angle): line for line in result.lines}
            assert cells_rotated[9, 0].text == "9"
            assert cells_rotated[9, 0].rotation_degrees == (360 - angle) % 360
        crop = rectify(np.asarray(image), np.asarray(cells[8, 2].quad), vertical=False)
        line = engine.recognize_line(Image.fromarray(crop))
        assert line.text == "9" and not line.rotated_180 and line.rotation_degrees == 0
        blank = engine.recognize_line(Image.new("RGB", (14, 14), "white"))
        assert blank.text == "" and blank.confidence is None
