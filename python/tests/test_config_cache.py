import json
import struct

import onnx
import pytest
from onnx import TensorProto, helper
from test_container import container_bytes

from oneocr_native.cache import prepare
from oneocr_native.config import PipelineConfig, fields
from oneocr_native.errors import ModelFormatError, UnsupportedModelError


def vi(value):
    out = bytearray()
    while value > 127:
        out.append((value & 127) | 128)
        value >>= 7
    out.append(value)
    return bytes(out)


def msg(number, value):
    if isinstance(value, str):
        value = value.encode()
    return vi(number * 8 + 2) + vi(len(value)) + value


def integer(number, value):
    return vi(number * 8) + vi(value)


def config_bytes(model_name="tiny.onnx"):
    pack = msg(1, msg(1, model_name))
    det = msg(1, "Universal") + msg(3, pack)
    for level in [2, 3, 4]:
        det += msg(9, msg(1, f"P{level}") + vi(2 * 8 + 5) + struct.pack("<f", 0.8))
    char = msg(1, "Latin") + msg(3, pack) + msg(5, "alphabet") + integer(7, 4)
    return msg(1, det) + msg(3, char) + msg(20, msg(1, "Aux") + msg(2, pack))


def tiny_model():
    graph = helper.make_graph(
        [helper.make_node("Identity", ["data"], ["result"])],
        "test",
        [helper.make_tensor_value_info("data", TensorProto.FLOAT, [1, 3, 4, 4])],
        [helper.make_tensor_value_info("result", TensorProto.FLOAT, [1, 3, 4, 4])],
    )
    return helper.make_model(
        graph, opset_imports=[helper.make_opsetid("", 17)], ir_version=9
    ).SerializeToString()


def write_model(path):
    path.write_bytes(
        container_bytes(
            [("tiny.onnx", tiny_model()), ("alphabet", b"a 0\n<blank> 1\n")], config=config_bytes()
        )
    )


def test_configuration_references_are_resolved():
    config = PipelineConfig.parse(config_bytes())
    assert config.characters[0].script == "Latin"
    assert config.required_paths() == {"tiny.onnx", "alphabet"}


@pytest.mark.parametrize("payload", [b"\x00", b"\x0a\xff", b"\x0a\x04x", b"\x08" + b"\xff" * 10])
def test_rejects_malformed_protobuf(payload):
    with pytest.raises(ModelFormatError):
        fields(payload)


def test_cache_is_reused_and_corruption_is_rebuilt(tmp_path):
    model = tmp_path / "source.onemodel"
    write_model(model)
    prepared = prepare(model, tmp_path / "cache")
    manifest_path = prepared.directory / "manifest.json"
    stamp = manifest_path.stat().st_mtime_ns
    assert prepare(model, tmp_path / "cache").directory == prepared.directory
    assert manifest_path.stat().st_mtime_ns == stamp
    prepared.path("tiny.onnx").write_bytes(b"damaged weights")
    repaired = prepare(model, tmp_path / "cache")
    assert repaired.path("tiny.onnx").read_bytes() == tiny_model()
    assert repaired.manifest["resources"][0]["validation"]["cpu_smoke_inference"] == "passed"


def test_cache_version_and_runtime_mismatch_are_rebuilt(tmp_path):
    model = tmp_path / "source.onemodel"
    write_model(model)
    prepared = prepare(model, tmp_path / "cache")
    path = prepared.directory / "manifest.json"
    manifest = json.loads(path.read_text())
    manifest["converter_version"] = "unsupported-old-version"
    path.write_text(json.dumps(manifest))
    rebuilt = prepare(model, tmp_path / "cache")
    assert rebuilt.manifest["converter_version"] != "unsupported-old-version"
    manifest = json.loads(path.read_text())
    manifest["onnxruntime_version"] = "old-runtime"
    path.write_text(json.dumps(manifest))
    assert prepare(model, tmp_path / "cache").manifest["onnxruntime_version"] != "old-runtime"


def test_unsupported_model_leaves_no_ready_cache(tmp_path):
    model = tmp_path / "source.onemodel"
    model.write_bytes(
        container_bytes(
            [("tiny.onnx", b"not an ONNX graph"), ("alphabet", b"a 0")], config=config_bytes()
        )
    )
    with pytest.raises(UnsupportedModelError):
        prepare(model, tmp_path / "cache")
    assert not list((tmp_path / "cache").rglob("manifest.json"))
    assert not list((tmp_path / "cache").rglob(".prepare-*"))


def test_missing_dependency_fails_before_cache_creation(tmp_path):
    model = tmp_path / "source.onemodel"
    model.write_bytes(container_bytes(config=config_bytes("absent.onnx")))
    with pytest.raises(ModelFormatError, match="missing"):
        prepare(model, tmp_path / "cache")
    assert not (tmp_path / "cache").exists()


def test_external_tensor_is_not_opened(tmp_path):
    graph = onnx.load_model_from_string(tiny_model())
    external = TensorProto(
        name="external", data_type=TensorProto.FLOAT, dims=[1], data_location=TensorProto.EXTERNAL
    )
    external.external_data.add(key="location", value="/should/not/be/read")
    graph.graph.initializer.append(external)
    model = tmp_path / "source.onemodel"
    model.write_bytes(
        container_bytes(
            [("tiny.onnx", graph.SerializeToString()), ("alphabet", b"a 0")], config=config_bytes()
        )
    )
    with pytest.raises(UnsupportedModelError, match="external"):
        prepare(model, tmp_path / "cache")
