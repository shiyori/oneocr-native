"""Retired detector conversion must not return through another GPU mode."""

import os
from pathlib import Path

import numpy as np
import onnx
import pytest
from onnx import TensorProto, helper
from test_adaptation import make_model, tensor

from oneocr_native.adaptation import adapt_bundle, adapt_model
from oneocr_native.errors import ModelFormatError
from oneocr_native.runtime import session

HEADS = [
    f"{head}_{direction}_fpn{level}"
    for level in (2, 3, 4)
    for direction in ("hori", "vert")
    for head in ("scores", "bbox_deltas", "link_scores")
]


@pytest.mark.parametrize("backend", ["cpu", "cuda", "directml"])
@pytest.mark.parametrize("quantization", ["grid", "relaxed"])
def test_detector_keeps_quantized_arithmetic(backend, quantization):
    shape = [1, 1, 1, 256]
    model = make_model(
        [
            helper.make_node("QuantizeLinear", ["data", "s", "z"], ["q"]),
            helper.make_node(
                "QLinearConv", ["q", "s", "z", "w", "ws", "wz", "s", "z", "b"], ["cq"]
            ),
            helper.make_node(
                "QLinearAdd",
                ["cq", "s", "z", "q", "s", "z", "s", "z"],
                ["aq"],
                domain="com.microsoft",
            ),
            helper.make_node(
                "QLinearSigmoid",
                ["aq", "s", "z", "ss", "sz"],
                ["sq"],
                domain="com.microsoft",
            ),
            helper.make_node("DequantizeLinear", ["sq", "ss", "sz"], ["result"]),
            *[helper.make_node("Identity", ["result"], [name]) for name in HEADS],
        ],
        [helper.make_tensor_value_info("data", TensorProto.FLOAT, shape)],
        [helper.make_tensor_value_info(name, TensorProto.FLOAT, shape) for name in HEADS],
        [
            tensor("s", 0.125, np.float32),
            tensor("z", 128, np.uint8),
            tensor("w", [[[[2]]]], np.int8),
            tensor("ws", [0.25], np.float32),
            tensor("wz", [0], np.int8),
            tensor("b", [-4], np.int32),
            tensor("ss", 1 / 256, np.float32),
            tensor("sz", 0, np.uint8),
        ],
    )
    candidate, info = adapt_model(
        model.SerializeToString(), backend, detector=True, quantization=quantization
    )
    restored = onnx.load_model_from_string(candidate)
    assert list(restored.graph.node) == list(model.graph.node)
    assert list(restored.graph.initializer) == list(model.graph.initializer)
    assert info["recipe_version"] == "v3-original-quantized-detector"
    assert not info["approximate"]
    assert info["converted_operators"] == {}
    feeds = {"data": ((np.arange(256, dtype=np.float32) - 128) * 0.125).reshape(shape)}
    for expected, actual in zip(
        session(model.SerializeToString(), 1).run(HEADS, feeds),
        session(candidate, 1).run(HEADS, feeds),
    ):
        np.testing.assert_array_equal(actual, expected)


@pytest.mark.integration
def test_original_detector_capture_stays_exact():
    root = os.environ.get("ONEOCR_DETECTOR_REGRESSION_DIR")
    if not root:
        pytest.skip("provide the private detector/input capture directory")
    root = Path(root)
    source = (root / "formal-cjk-source/models/detection/universal.onnx").read_bytes()
    feeds = {
        "data": np.fromfile(root / "universal/input0.bin", np.float32).reshape(1, 3, 160, 1024),
        "im_info": np.array([[160, 1024, 1]], np.float32),
    }
    expected = session(source, 1).run(HEADS, feeds)
    for backend in ("cpu", "cuda", "directml"):
        candidate, _ = adapt_model(source, backend, detector=True)
        for actual, reference in zip(session(candidate, 1).run(HEADS, feeds), expected):
            np.testing.assert_array_equal(actual, reference)


def test_coreml_adaptation_is_retired_before_reading_or_writing(tmp_path):
    with pytest.raises(ModelFormatError, match="CoreML model adaptation has been retired"):
        adapt_model(b"", "coreml")
    target = tmp_path / "candidate"
    with pytest.raises(ModelFormatError, match="CoreML model adaptation has been retired"):
        adapt_bundle(tmp_path / "missing-bundle", target, "coreml")
    assert not target.exists()
