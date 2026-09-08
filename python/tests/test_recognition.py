import sys

import numpy as np
import pytest

from oneocr_native.bidi import visual_to_logical
from oneocr_native.errors import ModelFormatError
from oneocr_native.recognition import Alphabet, normalize_line


def test_ctc_blank_is_dictionary_defined_and_repeated_letters_survive(tmp_path):
    path = tmp_path / "alphabet.txt"
    path.write_text("<space> 0\n! 1\na 2\n<blank> 3\n")
    alphabet = Alphabet(path)
    probabilities = np.full((7, 4), -12, np.float32)
    for frame, token in enumerate([2, 2, 3, 2, 0, 1, 3]):
        probabilities[frame, token] = -0.01
    assert alphabet.decode(probabilities)[0] == "aa !"


def test_unicode_composite_expansion(tmp_path):
    path, composite = tmp_path / "alphabet.txt", tmp_path / "composite.txt"
    path.write_text("<space> 0\n\U000f0000 1\n<blank> 2\n")
    composite.write_text("// a model-private glyph\n\U000f0000 e\u0301\n")
    assert Alphabet(path, composite).decode(np.array([[-20, 0, -20]], np.float32))[0] == "é"


def test_rejects_wrong_dictionary_and_nonfinite_logits(tmp_path):
    path = tmp_path / "alphabet.txt"
    path.write_text("a 0\n<blank> 1\n")
    with pytest.raises(ModelFormatError):
        Alphabet(path).decode(np.array([[0, np.nan]], np.float32))
    path.write_text("a 0\n<blank> 0\n")
    with pytest.raises(ModelFormatError):
        Alphabet(path)


@pytest.mark.skipif(sys.platform != "darwin", reason="uses macOS ICU")
@pytest.mark.parametrize(
    "visual,logical",
    [
        ("ملاعلاب ابحرم", "مرحبا بالعالم"),
        ("םלוע םולש", "שלום עולם"),
        ("123.45 ריחמ", "מחיר 123.45"),
        ("USD 123.45 رعسلا", "السعر 123.45 USD"),
    ],
)
def test_rtl_preserves_numeric_runs(visual, logical):
    assert visual_to_logical(visual) == logical


def test_normalization_retains_rgb_channels_and_stride():
    image = np.full((30, 81, 3), [0, 127, 255], np.uint8)
    data = normalize_line(image, stride=8)
    assert data.shape[:3] == (1, 3, 60)
    assert data.shape[-1] % 8 == 0
    np.testing.assert_allclose(data[0, :, 30, 50], [0, 127 / 255, 1])
    np.testing.assert_array_equal(data[0, :, 30, 0], [1, 1, 1])
