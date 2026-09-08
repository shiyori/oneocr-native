"""Detection-only integer-grid conversion; target EP equivalence needs validation.

Float tensors carry exact byte codes, not dequantized real values. Conv partials
are bounded integer dot products; this bound assumes true FP32 arithmetic, and
cannot certify a provider that internally uses reduced precision or transforms.
"""

from __future__ import annotations

import copy
import platform

import numpy as np
import onnx
import onnxruntime as ort
from onnx import TensorProto, helper, numpy_helper

from .adaptation import _Converter, _node_inputs
from .errors import ModelFormatError

RECIPE = "v2-detector-integer-grid"
FLOAT_INTEGER_LIMIT = 2**24


def reference_runtime() -> dict:
    return {
        "onnxruntime": ort.__version__,
        "system": platform.system(),
        "machine": platform.machine(),
        "provider": "CPUExecutionProvider",
    }


def lookup_table(node: onnx.NodeProto, tensors: dict[str, np.ndarray]) -> np.ndarray:
    """Use the installed CPU kernel, including its rounding/approximation rules."""
    binary = node.op_type == "QLinearAdd"
    slots = (0, 3) if binary else (0,)
    shape = [256, 256] if binary else [256]
    reference = copy.deepcopy(node)
    for i in range(len(reference.input)):
        reference.input[i] = f"input_{i}"
    reference.output[0] = "result"
    inputs = [
        helper.make_tensor_value_info(
            reference.input[s],
            TensorProto.UINT8,
            [256, 1] if s == 0 and binary else [1, 256] if binary else [256],
        )
        for s in slots
    ]
    constants = [
        numpy_helper.from_array(tensors[name], reference.input[i])
        for i, name in enumerate(node.input)
        if i not in slots
    ]
    graph = helper.make_graph(
        [reference],
        "quantization_reference",
        inputs,
        [helper.make_tensor_value_info(reference.output[0], TensorProto.UINT8, shape)],
        constants,
    )
    model = helper.make_model(
        graph,
        ir_version=10,
        opset_imports=[helper.make_opsetid("", 13), helper.make_opsetid("com.microsoft", 1)],
    )
    options = ort.SessionOptions()
    options.intra_op_num_threads = options.inter_op_num_threads = 1
    options.graph_optimization_level = ort.GraphOptimizationLevel.ORT_DISABLE_ALL
    options.log_severity_level = 3
    session = ort.InferenceSession(
        model.SerializeToString(), options, providers=["CPUExecutionProvider"]
    )
    values = np.arange(256, dtype=np.uint8)
    feeds = {reference.input[0]: values[:, None] if binary else values}
    if binary:
        feeds[reference.input[3]] = values[None, :]
    return session.run(None, feeds)[0].astype(np.float32).reshape(-1)


def channel_blocks(weight: np.ndarray, max_input: int) -> list[tuple[int, int]]:
    """Contiguous channel blocks whose absolute dot-product bounds fit FP32."""
    per_channel = np.abs(weight.astype(np.int64)).sum(axis=tuple(range(2, weight.ndim)))
    per_channel *= max_input
    blocks, start = [], 0
    bound = np.zeros(weight.shape[0], np.int64)
    for channel in range(weight.shape[1]):
        contribution = per_channel[:, channel]
        if np.any(contribution > FLOAT_INTEGER_LIMIT):
            raise ModelFormatError("single Conv channel exceeds exact FP32 accumulator bound")
        if np.any(bound + contribution > FLOAT_INTEGER_LIMIT):
            blocks.append((start, channel))
            start, bound = channel, np.zeros_like(bound)
        bound += contribution
    blocks.append((start, weight.shape[1]))
    return blocks


class _DetectorConverter(_Converter):
    def __init__(self, model: onnx.ModelProto, backend: str):
        super().__init__(model, backend, True)
        self.quantized: dict[str, tuple[np.ndarray, np.ndarray]] = {}
        self.split_convolutions = 0

    def constant(self, value, hint: str) -> str:
        name = self.name(hint)
        self.model.graph.initializer.append(numpy_helper.from_array(np.asarray(value), name))
        return name

    def activation(self, scale_name: str, zero_name: str) -> tuple[np.ndarray, np.ndarray]:
        scale, zero = self.tensor(scale_name), self.tensor(zero_name)
        if (
            scale.dtype != np.float32
            or scale.size != 1
            or zero.size != 1
            or zero.dtype != np.uint8
            or not np.isfinite(scale).all()
            or np.any(scale <= 0)
        ):
            raise ModelFormatError(
                "detector requires positive scalar FLOAT scale / UINT8 zero point"
            )
        return scale.reshape(()), zero.reshape(())

    def require(self, value: str, scale: str, zero: str) -> None:
        expected = self.activation(scale, zero)
        actual = self.quantized.get(value)
        if actual is None or any(not np.array_equal(a, b) for a, b in zip(actual, expected)):
            raise ModelFormatError(f"detector quantization contract mismatch: {value}")

    def mark(self, output: str, scale: str, zero: str) -> None:
        self.quantized[output] = self.activation(scale, zero)

    def slice_channels(self, value: str, start: int, end: int) -> str:
        return self.emit(
            "Slice",
            [
                value,
                self.constant(np.array([start], np.int64), "start"),
                self.constant(np.array([end], np.int64), "end"),
                self.constant(np.array([1], np.int64), "axis"),
            ],
        )

    def conv(self, n: onnx.NodeProto) -> None:
        if len(n.input) not in (8, 9):
            raise ModelFormatError("unsupported detector QLinearConv inputs")
        x, xs, xz, w, ws, wz, ys, yz = n.input[:8]
        self.require(x, xs, xz)
        xscale, xzero = self.activation(xs, xz)
        yscale, yzero = self.activation(ys, yz)
        weight, scale, zero = self.tensor(w), self.tensor(ws), self.tensor(wz)
        if (
            weight.dtype != np.int8
            or weight.ndim != 4
            or min(weight.shape) <= 0
            or scale.dtype != np.float32
            or scale.ndim > 1
            or scale.size not in (1, weight.shape[0])
            or zero.dtype != np.int8
            or zero.shape != scale.shape
            or not np.isfinite(scale).all()
            or np.any(scale <= 0)
        ):
            raise ModelFormatError("unsupported detector Conv weights / per-channel quantization")
        centered = weight.astype(np.int32) - zero.astype(np.int32).reshape(-1, 1, 1, 1)
        bias = (
            self.tensor(n.input[8])
            if len(n.input) == 9 and n.input[8]
            else np.zeros(weight.shape[0], np.int32)
        )
        if bias.dtype != np.int32 or bias.shape != (weight.shape[0],):
            raise ModelFormatError("unsupported detector Conv INT32 bias")
        max_input = max(int(xzero), 255 - int(xzero))
        bound = max_input * np.abs(centered.astype(np.int64)).sum(axis=(1, 2, 3))
        if np.any(bound + np.abs(bias.astype(np.int64)) > np.iinfo(np.int32).max):
            raise ModelFormatError("detector Conv may overflow INT32 accumulation")
        attrs = {a.name: helper.get_attribute_value(a) for a in n.attribute}
        group = attrs.pop("group", 1)
        if not isinstance(group, int) or group <= 0 or weight.shape[0] % group:
            raise ModelFormatError("invalid detector Conv groups")
        xcentered = self.emit("Sub", [x, self.constant(np.float32(xzero), "xzero")])
        group_results = []
        split = False
        outputs_per_group = weight.shape[0] // group
        for g in range(group):
            wg = centered[g * outputs_per_group : (g + 1) * outputs_per_group]
            blocks = channel_blocks(wg, max_input)
            split |= len(blocks) > 1
            acc = None
            for start, end in blocks:
                offset = g * weight.shape[1]
                chunk = (
                    xcentered
                    if group == 1 and len(blocks) == 1
                    else self.slice_channels(xcentered, offset + start, offset + end)
                )
                partial = self.emit(
                    "Conv",
                    [chunk, self.constant(wg[:, start:end].astype(np.float32), "integer_weight")],
                    **attrs,
                )
                partial = self.emit("Cast", [partial], to=TensorProto.INT32)
                acc = partial if acc is None else self.emit("Add", [acc, partial])
            group_results.append(acc)
        self.split_convolutions += int(split)
        acc = group_results[0] if group == 1 else self.emit("Concat", group_results, axis=1)
        acc = self.emit("Add", [acc, self.constant(bias.reshape(1, -1, 1, 1), "integer_bias")])
        acc = self.emit("Cast", [acc], to=TensorProto.FLOAT)
        # Keep the CPU kernel's float32 multiply then divide order for its multiplier.
        multiplier = ((xscale * scale) / yscale).astype(np.float32).reshape(1, -1, 1, 1)
        if not np.isfinite(multiplier).all():
            raise ModelFormatError("nonfinite detector Conv requantization multiplier")
        scaled = self.emit("Mul", [acc, self.constant(multiplier, "requant_scale")])
        rounded = self.emit("Round", [scaled])
        shifted = self.emit("Add", [rounded, self.constant(np.float32(yzero), "yzero")])
        self.emit(
            "Clip",
            [shifted, self.constant(np.float32(0), "qmin"), self.constant(np.float32(255), "qmax")],
            n.output[0],
        )
        self.mark(n.output[0], ys, yz)

    def lut(self, n: onnx.NodeProto) -> None:
        binary = n.op_type == "QLinearAdd"
        if len(n.input) != (8 if binary else 5) or len(n.output) != 1 or n.attribute:
            raise ModelFormatError("unsupported detector lookup operator layout")
        self.require(*n.input[:3])
        if binary:
            self.require(*n.input[3:6])
        self.activation(*n.input[-2:])
        index = self.emit("Cast", [n.input[0]], to=TensorProto.INT32)
        if binary:
            index = self.emit("Mul", [index, self.constant(np.int32(256), "lut_stride")])
            other = self.emit("Cast", [n.input[3]], to=TensorProto.INT32)
            index = self.emit("Add", [index, other])
        table = self.constant(lookup_table(n, self.tensors), "lookup")
        self.emit("Gather", [table, index], n.output[0], axis=0)
        self.mark(n.output[0], *n.input[-2:])

    def convert(self) -> None:
        for n in self.model.graph.node:
            op = n.op_type
            if op == "QuantizeLinear" and not n.domain:
                if len(n.input) != 3 or n.attribute or n.input[0] in self.quantized:
                    raise ModelFormatError("unsupported detector input quantization")
                self.activation(*n.input[1:3])
                quantizer = copy.deepcopy(n)
                quantizer.output[0] = self.name("input_byte")
                self.nodes.append(quantizer)
                self.emit("Cast", [quantizer.output[0]], n.output[0], to=TensorProto.FLOAT)
                self.mark(n.output[0], *n.input[1:3])
            elif op == "QLinearConv" and not n.domain:
                self.conv(n)
            elif op in ("QLinearAdd", "QLinearSigmoid") and n.domain == "com.microsoft":
                self.lut(n)
            elif op == "DequantizeLinear" and not n.domain and n.input[0] in self.quantized:
                if len(n.input) != 3 or n.attribute:
                    raise ModelFormatError("unsupported detector output dequantization")
                self.require(*n.input)
                integer = self.emit("Cast", [n.input[0]], to=TensorProto.UINT8)
                self.emit("DequantizeLinear", [integer, *n.input[1:]], n.output[0])
            else:
                consumed = [s for s in n.input if s in self.quantized]
                if consumed:
                    if (
                        n.domain
                        or op not in ("MaxPool", "Resize", "Identity")
                        or consumed != [n.input[0]]
                        or len(n.output) != 1
                    ):
                        raise ModelFormatError(f"unsupported detector quantized consumer: {op}")
                    if op == "Resize":
                        attrs = {a.name: helper.get_attribute_value(a) for a in n.attribute}
                        if (
                            attrs.get("mode", b"nearest") != b"nearest"
                            or attrs.get("coordinate_transformation_mode") != b"asymmetric"
                            or attrs.get("nearest_mode") != b"floor"
                        ):
                            raise ModelFormatError(
                                "detector Resize requires nearest/asymmetric/floor"
                            )
                    self.quantized[n.output[0]] = self.quantized[n.input[0]]
                # Subgraphs can capture outer-scope codes: never silently pass those through.
                if any(s in self.quantized for s in _node_inputs(n) - set(n.input)):
                    raise ModelFormatError("unsupported detector quantized subgraph capture")
                self.nodes.append(copy.deepcopy(n))
                continue
            self.converted[op] = self.converted.get(op, 0) + 1
        if any(v.name in self.quantized for v in self.model.graph.output):
            raise ModelFormatError("detector graph output must be dequantized")
        del self.model.graph.node[:]
        self.model.graph.node.extend(self.nodes)
