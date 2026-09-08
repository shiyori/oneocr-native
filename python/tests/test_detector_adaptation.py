"""Integer-grid contracts and target-CPU numerical regressions."""

import copy
import os
from pathlib import Path

import numpy as np
import onnx
import onnxruntime as ort
import pytest
from onnx import TensorProto, helper
from test_adaptation import make_model, tensor

from oneocr_native.adaptation import _prune, adapt_model
from oneocr_native.detector_adaptation import _DetectorConverter, channel_blocks
from oneocr_native.errors import ModelFormatError


def convert(model):
    candidate = copy.deepcopy(model)
    converter = _DetectorConverter(candidate, "coreml")
    converter.convert()
    _prune(candidate)
    onnx.checker.check_model(candidate)
    return candidate, converter


def run(model, feeds, optimized=False):
    options = ort.SessionOptions()
    options.intra_op_num_threads = options.inter_op_num_threads = 1
    options.graph_optimization_level = (
        ort.GraphOptimizationLevel.ORT_ENABLE_ALL
        if optimized
        else ort.GraphOptimizationLevel.ORT_DISABLE_ALL
    )
    options.log_severity_level = 3
    return ort.InferenceSession(
        model.SerializeToString(), options, providers=["CPUExecutionProvider"]
    ).run(None, feeds)


def conv_model(weight, bias, *, xs=0.019, xz=129, ws=0.0017, ys=0.027, yz=127, **attrs):
    weight = np.asarray(weight, np.int8)
    initial = [
        tensor("xs", xs, np.float32),
        tensor("xz", xz, np.uint8),
        tensor("w", weight, np.int8),
        tensor("ws", ws, np.float32),
        tensor("wz", np.zeros_like(ws), np.int8),
        tensor("ys", ys, np.float32),
        tensor("yz", yz, np.uint8),
        tensor("b", bias, np.int32),
    ]
    nodes = [
        helper.make_node("QuantizeLinear", ["data", "xs", "xz"], ["xq"]),
        helper.make_node(
            "QLinearConv", ["xq", "xs", "xz", "w", "ws", "wz", "ys", "yz", "b"], ["yq"], **attrs
        ),
        helper.make_node("DequantizeLinear", ["yq", "ys", "yz"], ["result"]),
    ]
    return make_model(
        nodes,
        [
            helper.make_tensor_value_info(
                "data", TensorProto.FLOAT, [1, weight.shape[1] * attrs.get("group", 1), None, None]
            )
        ],
        [
            helper.make_tensor_value_info(
                "result", TensorProto.FLOAT, [1, weight.shape[0], None, None]
            )
        ],
        initial,
    )


@pytest.mark.parametrize("group", [1, 2])
@pytest.mark.parametrize("optimized", [False, True])
def test_integer_conv_random_padding_stride_groups(group, optimized):
    rng = np.random.default_rng(726)
    model = conv_model(
        rng.integers(-100, 101, (8, 16, 3, 3)),
        rng.integers(-900, 901, 8),
        ws=np.linspace(0.0017, 0.0041, 8, dtype=np.float32),
        group=group,
        pads=[1, 1, 1, 1],
        strides=[2, 2],
    )
    candidate, _ = convert(model)
    values = rng.uniform(-3, 3, (1, 16 * group, 13, 17)).astype(np.float32)
    np.testing.assert_array_equal(
        run(candidate, {"data": values}, optimized)[0], run(model, {"data": values}, optimized)[0]
    )


@pytest.mark.parametrize("yz", [0, 1, 127, 128, 129, 255])
def test_halfway_rounding_zero_points_bias_and_clipping(yz):
    model = conv_model([[[[1]]]], [1], xs=0.125, xz=128, ws=0.25, ys=0.0625, yz=yz)
    candidate, _ = convert(model)
    grid = (np.arange(256, dtype=np.float32) - 128) * 0.125
    # Includes exact output half-ties, input half-ties and adjacent representable inputs.
    values = np.concatenate(
        [
            grid,
            grid + 0.0625,
            np.nextafter(grid + 0.0625, np.inf),
            np.nextafter(grid + 0.0625, -np.inf),
        ]
    ).reshape(1, 1, 1, -1)
    np.testing.assert_array_equal(
        run(candidate, {"data": values})[0], run(model, {"data": values})[0]
    )


def test_split_accumulation_and_int32_bias():
    weight = np.full((2, 600, 3, 3), 127, np.int8)
    model = conv_model(
        weight, [16777217, -16777217], xs=0.019, xz=129, ws=[0.0017, 0.0041], ys=10000
    )
    candidate, converter = convert(model)
    assert converter.split_convolutions == 1
    values = np.full((1, 600, 3, 3), -2, np.float32)
    np.testing.assert_array_equal(
        run(candidate, {"data": values})[0], run(model, {"data": values})[0]
    )
    assert len(channel_blocks(weight, 129)) > 1


def test_reject_overflow_and_contract_mismatch():
    model = conv_model([[[[127]]]], [np.iinfo(np.int32).max])
    with pytest.raises(ModelFormatError, match="overflow"):
        convert(model)
    model = conv_model([[[[1]]]], [0])
    model.graph.initializer.append(tensor("wrong", 0.04, np.float32))
    model.graph.node[1].input[1] = "wrong"
    with pytest.raises(ModelFormatError, match="contract mismatch"):
        convert(model)


@pytest.mark.parametrize("binary", [False, True])
@pytest.mark.parametrize("optimized", [False, True])
def test_lookup_exhaustive(binary, optimized):
    initial = [
        tensor("as", 0.019, np.float32),
        tensor("az", 129, np.uint8),
        tensor("bs", 0.031, np.float32),
        tensor("bz", 117, np.uint8),
        tensor("ys", 0.017 if binary else 1 / 255, np.float32),
        tensor("yz", 127 if binary else 0, np.uint8),
    ]
    nodes = [helper.make_node("QuantizeLinear", ["a", "as", "az"], ["aq"])]
    inputs = [helper.make_tensor_value_info("a", TensorProto.FLOAT, [None, None])]
    if binary:
        nodes.append(helper.make_node("QuantizeLinear", ["b", "bs", "bz"], ["bq"]))
        inputs.append(helper.make_tensor_value_info("b", TensorProto.FLOAT, [None, None]))
    nodes.append(
        helper.make_node(
            "QLinearAdd" if binary else "QLinearSigmoid",
            ["aq", "as", "az", "bq", "bs", "bz", "ys", "yz"]
            if binary
            else ["aq", "as", "az", "ys", "yz"],
            ["yq"],
            domain="com.microsoft",
        )
    )
    nodes.append(helper.make_node("DequantizeLinear", ["yq", "ys", "yz"], ["result"]))
    model = make_model(
        nodes,
        inputs,
        [helper.make_tensor_value_info("result", TensorProto.FLOAT, [None, None])],
        initial,
    )
    candidate, _ = convert(model)
    feeds = {"a": ((np.arange(256, dtype=np.float32) - 129) * np.float32(0.019))[:, None]}
    if binary:
        feeds["b"] = ((np.arange(256, dtype=np.float32) - 117) * np.float32(0.031))[None, :]
    np.testing.assert_array_equal(
        run(candidate, feeds, optimized)[0], run(model, feeds, optimized)[0]
    )
    if binary:
        # Elementwise/vector kernels and broadcasting must agree with the oracle table.
        expanded = {k: np.broadcast_to(v, (256, 256)).copy() for k, v in feeds.items()}
        np.testing.assert_array_equal(
            run(candidate, expanded, optimized)[0], run(model, expanded, optimized)[0]
        )
        add = next(n for n in model.graph.node if n.op_type == "QLinearAdd")
        add.input[3:6] = add.input[:3]
        candidate, _ = convert(model)
        np.testing.assert_array_equal(
            run(candidate, {"a": feeds["a"]}, optimized)[0], run(model, feeds, optimized)[0]
        )


def test_reject_unknown_consumer_and_interpolating_resize():
    model = conv_model([[[[1]]]], [0])
    model.graph.node.insert(2, helper.make_node("Relu", ["yq"], ["bad"]))
    with pytest.raises(ModelFormatError, match="consumer"):
        convert(model)
    model.graph.node[2].CopyFrom(
        helper.make_node("Resize", ["yq", "", "scales"], ["bad"], mode="linear")
    )
    model.graph.initializer.append(tensor("scales", [1, 1, 2, 2], np.float32))
    with pytest.raises(ModelFormatError, match="Resize"):
        convert(model)


@pytest.mark.integration
def test_real_first_divergent_conv_and_detector():
    """User-supplied private model/input only; no proprietary tensors enter the repo."""
    root = os.environ.get("ONEOCR_DETECTOR_REGRESSION_DIR")
    if not root:
        pytest.skip("set ONEOCR_DETECTOR_REGRESSION_DIR to the captured regression directory")
    root = Path(root)
    source = (root / "formal-cjk-source/models/detection/universal.onnx").read_bytes()
    original = onnx.load_model_from_string(source)
    feeds = {
        "data": np.fromfile(root / "universal/input0.bin", np.float32).reshape(1, 3, 160, 1024),
        "im_info": np.array([[160, 1024, 1]], np.float32),
    }
    # Preserve the first observed divergence as an explicitly inspected intermediate.
    probe = "pytorch_1790_quantized"
    original.graph.output.append(helper.make_tensor_value_info(probe, TensorProto.UINT8, None))
    converted, info = adapt_model(source, "coreml", detector=True)
    candidate = onnx.load_model_from_string(converted)
    candidate.graph.output.append(helper.make_tensor_value_info(probe, TensorProto.FLOAT, None))
    names = [o.name for o in candidate.graph.output]
    values = {v.name: v for v in original.graph.output}
    del original.graph.output[:]
    original.graph.output.extend(values[name] for name in names)
    assert info["split_convolutions"] == 2
    assert info["recipe_version"] == "v2-detector-integer-grid"
    for optimized in (False, True):
        for expected, actual in zip(
            run(original, feeds, optimized), run(candidate, feeds, optimized)
        ):
            np.testing.assert_array_equal(actual, expected)
