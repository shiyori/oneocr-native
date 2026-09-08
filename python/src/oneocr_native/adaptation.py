"""Offline, opt-in ONNX candidates for the native SDK.

The original resources are never overwritten. Conversion is a developer step;
Go/C/C++ applications do not require Python. Converted quantized arithmetic is
experimental even when activation rounding is retained: floating accumulation
and a standard LSTM need numerical and end-to-end validation on each device.
"""

from __future__ import annotations

import copy
import json
import shutil
import tempfile
from hashlib import sha256
from pathlib import Path

import numpy as np
import onnx
from onnx import TensorProto, helper, numpy_helper

from .bundle import load_bundle
from .errors import ModelFormatError

SCHEMA = "oneocr.adaptation.v1"
BACKENDS = ("cpu", "cuda", "directml")


def _external_inputs(graph: onnx.GraphProto) -> set[str]:
    local = {v.name for v in graph.input} | {t.name for t in graph.initializer}
    local |= {s for n in graph.node for s in n.output}
    return {s for n in graph.node for s in _node_inputs(n) if s not in local}


def _node_inputs(node: onnx.NodeProto) -> set[str]:
    names = {s for s in node.input if s}
    for attr in node.attribute:
        if attr.type == onnx.AttributeProto.GRAPH:
            names |= _external_inputs(attr.g)
        elif attr.type == onnx.AttributeProto.GRAPHS:
            for graph in attr.graphs:
                names |= _external_inputs(graph)
    return names


def _prune(model: onnx.ModelProto) -> None:
    needed = {v.name for v in model.graph.output}
    kept = []
    for node in reversed(model.graph.node):
        if needed.intersection(node.output):
            kept.append(node)
            needed |= _node_inputs(node)
    del model.graph.node[:]
    model.graph.node.extend(reversed(kept))
    tensors = [t for t in model.graph.initializer if t.name in needed]
    del model.graph.initializer[:]
    model.graph.initializer.extend(tensors)
    inputs = [
        v
        for v in model.graph.input
        if v.name in needed or v.name in ("data", "im_info", "seq_lengths", "allowed_tokens")
    ]
    del model.graph.input[:]
    model.graph.input.extend(inputs)
    # Old integer/intermediate annotations are invalid after float conversion.
    del model.graph.value_info[:]


class _Converter:
    def __init__(self, model: onnx.ModelProto, backend: str, grid: bool):
        self.model, self.backend, self.grid = model, backend, grid
        self.tensors = {t.name: numpy_helper.to_array(t) for t in model.graph.initializer}
        self.names = set(self.tensors) | {v.name for v in model.graph.input}
        self.names |= {s for n in model.graph.node for s in n.output}
        self.counter = 0
        self.nodes: list[onnx.NodeProto] = []
        self.floating: set[str] = set()
        self.converted: dict[str, int] = {}

    def name(self, hint: str) -> str:
        while True:
            self.counter += 1
            name = f"oneocr_adapt_{self.counter}_{hint}"
            if name not in self.names:
                self.names.add(name)
                return name

    def constant(self, value, hint: str) -> str:
        name = self.name(hint)
        data = np.asarray(value, dtype=np.float32)
        self.model.graph.initializer.append(numpy_helper.from_array(data, name))
        return name

    def tensor(self, name: str) -> np.ndarray:
        if name not in self.tensors:
            raise ModelFormatError(f"adaptation requires a constant quantization parameter: {name}")
        return self.tensors[name]

    def emit(self, op: str, inputs: list[str], output: str | None = None, **attrs) -> str:
        output = output or self.name(op)
        self.nodes.append(helper.make_node(op, inputs, [output], **attrs))
        return output

    def grid_output(self, value: str, scale_name: str, zero_name: str, output: str) -> None:
        scale, zero = self.tensor(scale_name), self.tensor(zero_name)
        if scale.size != 1 or zero.size != 1 or zero.dtype not in (np.int8, np.uint8):
            raise ModelFormatError("only scalar INT8/UINT8 activation quantization is supported")
        scale = float(scale.reshape(-1)[0])
        if not np.isfinite(scale) or scale <= 0:
            raise ModelFormatError("invalid activation scale")
        zp = int(zero.reshape(-1)[0])
        limits = np.iinfo(zero.dtype)
        if self.grid:
            divided = self.emit("Div", [value, self.constant(scale, "scale")])
            rounded = self.emit("Round", [divided])
            clipped = self.emit(
                "Clip",
                [
                    rounded,
                    self.constant(limits.min - zp, "qmin"),
                    self.constant(limits.max - zp, "qmax"),
                ],
            )
            self.emit("Mul", [clipped, self.constant(scale, "scale")], output)
        else:
            # Clipping must remain: zero-point=0 often encodes a fused ReLU.
            self.emit(
                "Clip",
                [
                    value,
                    self.constant((limits.min - zp) * scale, "min"),
                    self.constant((limits.max - zp) * scale, "max"),
                ],
                output,
            )
        self.floating.add(output)

    def dequant_weight(self, name: str, scale_name: str, zero_name: str, axis: int) -> np.ndarray:
        weight = self.tensor(name)
        scale, zero = self.tensor(scale_name), self.tensor(zero_name)
        if weight.dtype not in (np.int8, np.uint8):
            raise ModelFormatError("unsupported quantized weight dtype")
        if scale.size == 1 and zero.size == 1:
            return (
                weight.astype(np.float32) - zero.astype(np.float32).reshape(())
            ) * scale.reshape(())
        if scale.ndim != 1 or scale.size != weight.shape[axis] or zero.shape != scale.shape:
            raise ModelFormatError("unsupported per-channel weight layout")
        shape = [1] * weight.ndim
        shape[axis] = scale.size
        return (weight.astype(np.float32) - zero.astype(np.float32).reshape(shape)) * scale.reshape(
            shape
        )

    def lstm(self, n: onnx.NodeProto) -> None:
        if len(n.input) != 12:
            raise ModelFormatError("unsupported DynamicQuantizeLSTM input layout")
        attrs = {a.name: helper.get_attribute_value(a) for a in n.attribute}
        hidden = attrs.get("hidden_size")
        if not isinstance(hidden, int) or hidden <= 0:
            raise ModelFormatError("missing LSTM hidden size")
        replacement = list(n.input[:8])
        for slot, scale_slot, zero_slot in ((1, 8, 9), (2, 10, 11)):
            weight = self.tensor(n.input[slot])
            scale, zero = self.tensor(n.input[scale_slot]), self.tensor(n.input[zero_slot])
            if (
                weight.ndim != 3
                or weight.shape[2] != 4 * hidden
                or weight.dtype not in (np.int8, np.uint8)
            ):
                raise ModelFormatError("unsupported quantized LSTM weights")
            if scale.shape == (weight.shape[0],):
                shape = (weight.shape[0], 1, 1)
            elif scale.shape == (weight.shape[0], 4 * hidden):
                shape = (weight.shape[0], 1, 4 * hidden)
            else:
                raise ModelFormatError("unsupported LSTM scales")
            if zero.shape != scale.shape:
                raise ModelFormatError("LSTM scale/zero-point mismatch")
            # Contrib layout is [direction,input_or_hidden,4H]. Both contrib
            # and standard ONNX use i,o,f,c gates; only transpose W/R axes.
            value = (
                weight.astype(np.float32) - zero.astype(np.float32).reshape(shape)
            ) * scale.reshape(shape)
            replacement[slot] = self.constant(
                np.ascontiguousarray(value.transpose(0, 2, 1)), "lstm_weight"
            )
        self.nodes.append(helper.make_node("LSTM", replacement, list(n.output), **attrs))

    def convert(self) -> None:
        floats = self.backend == "cuda"
        for n in self.model.graph.node:
            op = n.op_type
            if (
                op == "DynamicQuantizeLSTM"
                and n.domain == "com.microsoft"
                and self.backend in ("cuda", "directml")
            ):
                self.lstm(n)
            elif floats and op == "QuantizeLinear" and n.domain == "":
                self.grid_output(n.input[0], n.input[1], n.input[2], n.output[0])
            elif floats and op == "QLinearConv" and n.domain == "":
                x, xs, _, w, ws, wz, ys, yz, bias = n.input
                if x not in self.floating:
                    raise ModelFormatError("quantized Conv input has no float conversion")
                weight = self.dequant_weight(w, ws, wz, 0)
                b = self.tensor(bias)
                scale = self.tensor(xs) * self.tensor(ws)
                if b.dtype != np.int32 or b.shape != (weight.shape[0],):
                    raise ModelFormatError("unsupported quantized Conv bias")
                result = self.name("conv")
                conv = helper.make_node(
                    "Conv",
                    [
                        x,
                        self.constant(weight, "weight"),
                        self.constant(b.astype(np.float32) * scale, "bias"),
                    ],
                    [result],
                )
                conv.attribute.extend(n.attribute)
                self.nodes.append(conv)
                self.grid_output(result, ys, yz, n.output[0])
            elif floats and op == "QLinearAdd" and n.domain == "com.microsoft":
                a, _, _, b, _, _, ys, yz = n.input
                if a not in self.floating or b not in self.floating:
                    raise ModelFormatError("quantized Add input has no float conversion")
                result = self.emit("Add", [a, b])
                self.grid_output(result, ys, yz, n.output[0])
            elif floats and op == "QLinearSigmoid" and n.domain == "com.microsoft":
                x, _, _, ys, yz = n.input
                if x not in self.floating:
                    raise ModelFormatError("quantized Sigmoid input has no float conversion")
                result = self.emit("Sigmoid", [x])
                self.grid_output(result, ys, yz, n.output[0])
            elif floats and op == "DequantizeLinear" and n.input[0] in self.floating:
                self.emit("Identity", [n.input[0]], n.output[0])
            elif floats and op == "MatMulIntegerToFloat" and n.domain == "com.microsoft":
                a, b, _, bs, _, bz, *bias = n.input
                if a not in self.floating:
                    raise ModelFormatError("quantized MatMul input has no float conversion")
                weight = self.dequant_weight(b, bs, bz, 1)
                inputs = [a, self.constant(weight, "projection")]
                if bias and bias[0]:
                    result = self.emit("MatMul", inputs)
                    self.emit("Add", [result, bias[0]], n.output[0])
                else:
                    self.emit("MatMul", inputs, n.output[0])
            else:
                if floats and any(s in self.floating for s in n.input):
                    if op not in ("MaxPool", "Resize", "Identity"):
                        raise ModelFormatError(
                            f"unsupported consumer of converted quantized values: {op}"
                        )
                    self.floating.update(n.output)
                self.nodes.append(copy.deepcopy(n))
                continue
            self.converted[op] = self.converted.get(op, 0) + 1
        del self.model.graph.node[:]
        self.model.graph.node.extend(self.nodes)


def compact_recognizer(model: onnx.ModelProto) -> None:
    if len(model.graph.output) != 1 or model.graph.output[0].name != "logsoftmax":
        raise ModelFormatError("compact output requires the original logsoftmax interface")
    classes = model.graph.output[0].type.tensor_type.shape.dim[-1].dim_value
    if classes <= 1:
        raise ModelFormatError("compact recognizer requires a fixed dictionary size")
    used = {s for n in model.graph.node for s in n.output}
    reserved = {
        "allowed_tokens",
        "token_ids",
        "invalid",
        "oneocr_masked",
        "oneocr_min_float",
        "oneocr_nan",
        "oneocr_inf",
        "oneocr_bad",
        "oneocr_bad_int",
    }
    if used & reserved or {v.name for v in model.graph.input} & reserved:
        raise ModelFormatError("compact output name collision")
    model.graph.input.append(
        helper.make_tensor_value_info("allowed_tokens", TensorProto.BOOL, [classes])
    )
    model.graph.initializer.append(
        numpy_helper.from_array(np.array(-np.inf, np.float32), "oneocr_min_float")
    )
    model.graph.node.extend(
        [
            helper.make_node(
                "Where", ["allowed_tokens", "logsoftmax", "oneocr_min_float"], ["oneocr_masked"]
            ),
            helper.make_node(
                "ArgMax", ["oneocr_masked"], ["token_ids"], axis=2, keepdims=0, select_last_index=0
            ),
            helper.make_node("IsNaN", ["logsoftmax"], ["oneocr_nan"]),
            helper.make_node("IsInf", ["logsoftmax"], ["oneocr_inf"]),
            helper.make_node("Or", ["oneocr_nan", "oneocr_inf"], ["oneocr_bad"]),
            helper.make_node("Cast", ["oneocr_bad"], ["oneocr_bad_int"], to=TensorProto.INT32),
            helper.make_node("ReduceMax", ["oneocr_bad_int"], ["invalid"], keepdims=1),
        ]
    )
    del model.graph.output[:]
    model.graph.output.extend(
        [
            helper.make_tensor_value_info("token_ids", TensorProto.INT64, [None, 1]),
            helper.make_tensor_value_info("invalid", TensorProto.INT32, [1, 1, 1]),
        ]
    )


def adapt_model(
    data: bytes,
    backend: str,
    *,
    detector: bool = False,
    compact: bool = False,
    quantization: str = "grid",
) -> tuple[bytes, dict]:
    if backend == "coreml":
        raise ModelFormatError("CoreML model adaptation has been retired; use original models")
    if backend not in BACKENDS or quantization not in ("grid", "relaxed"):
        raise ValueError("invalid adaptation backend or quantization mode")
    model = onnx.load_model_from_string(data)
    version = next((o.version for o in model.opset_import if not o.domain), 0)
    if version != 13:
        raise ModelFormatError("this adapter is validated for OneOCR ONNX opset 13 only")
    if detector:
        order = [
            f"{head}_{direction}_fpn{level}"
            for level in (2, 3, 4)
            for direction in ("hori", "vert")
            for head in ("scores", "bbox_deltas", "link_scores")
        ]
        values = {v.name: v for v in model.graph.output}
        if any(name not in values for name in order):
            raise ModelFormatError("unexpected detector interface")
        del model.graph.output[:]
        model.graph.output.extend(values[name] for name in order)
    elif {"script_id_score", "flip_score"}.issubset(v.name for v in model.graph.output):
        values = {v.name: v for v in model.graph.output}
        del model.graph.output[:]
        model.graph.output.extend(values[name] for name in ("script_id_score", "flip_score"))
    _prune(model)
    # The integer-grid detector recipe was retired. Preserve the original
    # quantized operators instead of silently reviving the inaccurate v1
    # floating-point detector conversion, including in relaxed mode.
    recipe = "v3-original-quantized-detector" if detector else "v1"
    converter = _Converter(model, "cpu" if detector else backend, quantization == "grid")
    converter.convert()
    if compact:
        compact_recognizer(model)
    _prune(model)
    onnx.checker.check_model(model)
    # The runtime cache is also isolated by a hash of these final bytes.
    return model.SerializeToString(), {
        "outputs": [v.name for v in model.graph.output],
        "converted_operators": converter.converted,
        "approximate": bool(converter.converted),
        "recipe_version": recipe,
    }


def adapt_bundle(
    bundle_dir: str | Path,
    output_dir: str | Path,
    backend: str,
    *,
    compact: bool = False,
    quantization: str = "grid",
) -> Path:
    if backend == "coreml":
        raise ModelFormatError("CoreML model adaptation has been retired; use original models")
    if backend not in BACKENDS:
        raise ValueError("unknown adaptation backend")
    prepared = load_bundle(bundle_dir)
    source = Path(bundle_dir)
    manifest = json.loads((source / "bundle.json").read_text(encoding="utf-8"))
    target = Path(output_dir).absolute()
    if target.exists():
        raise ModelFormatError(f"adaptation destination already exists: {target}")
    target.parent.mkdir(parents=True, exist_ok=True)
    temporary = Path(tempfile.mkdtemp(prefix=".oneocr-adapt-", dir=target.parent))
    resources = {r["file"]: r for r in manifest["resources"]}
    models = [
        (prepared.config.detector_path, "detector", True, False),
        (prepared.config.classifier_path, "classifier", False, False),
    ]
    models += [
        (c.model_path, f"recognizer-{c.script}", False, compact)
        for c in prepared.config.characters
        if c.script in ("CJK", "Latin")
    ]
    try:
        entries = {}
        for original, stage, is_detector, use_compact in models:
            data = (source / original).read_bytes()
            result, details = adapt_model(
                data, backend, detector=is_detector, compact=use_compact, quantization=quantization
            )
            filename = f"models/{stage}.onnx"
            path = temporary / filename
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(result)
            entries[original] = {
                "file": filename,
                "bytes": len(result),
                "sha256": sha256(result).hexdigest(),
                "source_sha256": resources[original]["sha256"],
                "recipe": f"{details['recipe_version']}-{backend}-{quantization}"
                + ("-compact" if use_compact else ""),
                **details,
            }
        result = {
            "schema": SCHEMA,
            "source_sha256": manifest["source_sha256"],
            "backend": backend,
            "models": entries,
            "validation": "experimental; validate accuracy and actual EP execution on target hardware",
        }
        (temporary / "adaptation.json").write_text(
            json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
        )
        temporary.rename(target)
    finally:
        if temporary.exists():
            shutil.rmtree(temporary)
    return target
