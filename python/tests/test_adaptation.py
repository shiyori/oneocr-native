import numpy as np
import onnx
import pytest
from onnx import TensorProto, helper, numpy_helper

from oneocr_native.adaptation import adapt_model
from oneocr_native.runtime import session


def make_model(nodes, inputs, outputs, tensors=()):
    return helper.make_model(
        helper.make_graph(nodes, "test", inputs, outputs, initializer=list(tensors)),
        opset_imports=[helper.make_opsetid("", 13), helper.make_opsetid("com.microsoft", 1)],
        ir_version=10,
    )


def tensor(name, value, dtype):
    return numpy_helper.from_array(np.array(value, dtype=dtype), name)


def test_compact_preserves_mask_ties_and_complete_nonfinite_check():
    model = make_model(
        [helper.make_node("Identity", ["data"], ["logsoftmax"])],
        [helper.make_tensor_value_info("data", TensorProto.FLOAT, [3, 1, 4])],
        [helper.make_tensor_value_info("logsoftmax", TensorProto.FLOAT, [3, 1, 4])],
    )
    data, info = adapt_model(model.SerializeToString(), "cpu", compact=True)
    assert not info["approximate"]
    compiled = session(data, 1)
    values = np.array([[[3, 2, 2, 0]], [[1, 5, 5, 0]], [[0, 0, 0, 0]]], np.float32)
    allowed = np.array([False, True, True, True])
    ids, invalid = compiled.run(None, {"data": values, "allowed_tokens": allowed})
    np.testing.assert_array_equal(ids, [[1], [1], [1]])
    assert invalid.shape == (1, 1, 1) and invalid.item() == 0
    extreme = np.full_like(values, np.finfo(np.float32).min)
    ids, invalid = compiled.run(None, {"data": extreme, "allowed_tokens": allowed})
    np.testing.assert_array_equal(ids, [[1], [1], [1]])
    assert invalid.item() == 0
    for bad in (np.nan, np.inf, -np.inf):
        broken = values.copy()
        broken[0, 0, 0] = bad  # Excluded token must still be checked.
        _, invalid = compiled.run(None, {"data": broken, "allowed_tokens": allowed})
        assert invalid.item() == 1


def test_quantized_conv_grid_retains_clipping_and_bias():
    initial = [
        tensor("xs", 0.125, np.float32),
        tensor("xz", 128, np.uint8),
        tensor("w", [[[[2]]]], np.int8),
        tensor("ws", [0.25], np.float32),
        tensor("wz", [0], np.int8),
        tensor("ys", 0.125, np.float32),
        tensor("yz", 0, np.uint8),
        tensor("b", [-4], np.int32),
    ]
    nodes = [
        helper.make_node("QuantizeLinear", ["data", "xs", "xz"], ["xq"]),
        helper.make_node(
            "QLinearConv", ["xq", "xs", "xz", "w", "ws", "wz", "ys", "yz", "b"], ["yq"]
        ),
        helper.make_node("DequantizeLinear", ["yq", "ys", "yz"], ["result"]),
    ]
    model = make_model(
        nodes,
        [helper.make_tensor_value_info("data", TensorProto.FLOAT, [1, 1, 2, 4])],
        [helper.make_tensor_value_info("result", TensorProto.FLOAT, [1, 1, 2, 4])],
        initial,
    )
    values = np.array([-2, -1, 0, 0.25, 0.5, 1, 4, 40], np.float32).reshape(1, 1, 2, 4)
    expected = session(model.SerializeToString(), 1).run(None, {"data": values})[0]
    converted, info = adapt_model(model.SerializeToString(), "cuda")
    got = session(converted, 1).run(None, {"data": values})[0]
    np.testing.assert_array_equal(got, expected)
    assert got.min() == 0  # Removing quantizers without clipping loses fused ReLU.
    assert info["converted_operators"]["QLinearConv"] == 1


def test_standard_lstm_keeps_direction_gate_and_weight_layout():
    rng = np.random.default_rng(726)
    initial = [
        tensor("w", rng.integers(-5, 6, (2, 3, 8)), np.int8),
        tensor("r", rng.integers(-5, 6, (2, 2, 8)), np.int8),
        tensor("b", rng.normal(0, 0.1, (2, 16)), np.float32),
        tensor("ws", np.linspace(0.02, 0.12, 16).reshape(2, 8), np.float32),
        tensor("wz", np.zeros((2, 8)), np.int8),
        tensor("rs", np.linspace(0.03, 0.15, 16).reshape(2, 8), np.float32),
        tensor("rz", np.zeros((2, 8)), np.int8),
    ]
    model = make_model(
        [
            helper.make_node(
                "DynamicQuantizeLSTM",
                ["data", "w", "r", "b", "", "", "", "", "ws", "wz", "rs", "rz"],
                ["result"],
                domain="com.microsoft",
                hidden_size=2,
                direction="bidirectional",
            )
        ],
        [helper.make_tensor_value_info("data", TensorProto.FLOAT, [5, 1, 3])],
        [helper.make_tensor_value_info("result", TensorProto.FLOAT, [5, 2, 1, 2])],
        initial,
    )
    values = rng.normal(0, 0.5, (5, 1, 3)).astype(np.float32)
    expected = session(model.SerializeToString(), 1).run(None, {"data": values})[0]
    converted, info = adapt_model(model.SerializeToString(), "cuda")
    got = session(converted, 1).run(None, {"data": values})[0]
    np.testing.assert_allclose(got, expected, atol=0.015, rtol=0.015)
    assert info["approximate"]
    assert "DynamicQuantizeLSTM" not in {
        n.op_type for n in onnx.load_model_from_string(converted).graph.node
    }
    assert got.shape == (5, 2, 1, 2)


def test_unknown_backend_does_not_silently_generate_cpu_model():
    with pytest.raises(ValueError):
        adapt_model(b"", "gpu")
