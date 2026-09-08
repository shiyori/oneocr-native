import json
import zipfile
from pathlib import Path

import pytest
from test_config_cache import write_model

from oneocr_native.bundle import export_bundle, load_bundle
from oneocr_native.errors import ModelFormatError


def test_portable_export_preserves_resources_and_interfaces(tmp_path):
    model = tmp_path / "source.onemodel"
    write_model(model)
    output = export_bundle(
        model, tmp_path / "bundle", archive=tmp_path / "bundle.zip", cache_dir=tmp_path / "cache"
    )
    prepared = load_bundle(output)
    manifest = json.loads((output / "bundle.json").read_text())
    assert len(manifest["resources"]) == 2
    onnx = next(r for r in manifest["resources"] if r["kind"] == "onnx")
    assert onnx["interface"]["inputs"] == [
        {"name": "data", "dtype": "FLOAT", "shape": [1, 3, 4, 4]}
    ]
    assert onnx["interface"]["operators"] == ["ai.onnx::Identity"]
    assert prepared.path(prepared.config.detector_path).is_file()
    with zipfile.ZipFile(tmp_path / "bundle.zip") as archive:
        assert set(archive.namelist()) == {
            p.relative_to(output).as_posix() for p in output.rglob("*") if p.is_file()
        }
    with pytest.raises(ModelFormatError, match="already exists"):
        export_bundle(model, output, cache_dir=tmp_path / "cache")


@pytest.mark.parametrize("change", ["hash", "escape", "missing", "duplicate"])
def test_bundle_rejects_corruption(tmp_path, change):
    model = tmp_path / "source.onemodel"
    write_model(model)
    output = export_bundle(model, tmp_path / "bundle", cache_dir=tmp_path / "cache")
    file = output / "bundle.json"
    manifest = json.loads(file.read_text())
    if change == "hash":
        manifest["resources"][0]["sha256"] = "0" * 64
    elif change == "escape":
        manifest["resources"][0]["file"] = "../source.onemodel"
    elif change == "missing":
        manifest["pipeline"]["detector_path"] = "missing.onnx"
    else:
        manifest["resources"].append(manifest["resources"][0])
    file.write_text(json.dumps(manifest))
    with pytest.raises(ModelFormatError):
        load_bundle(Path(output))


def test_standard_resource_names_do_not_depend_on_checkpoints():
    from oneocr_native.resource_names import resource_filename

    assert (
        resource_filename(
            r"C:\build\Model_Edge\Character\LatinPrintedV2\ONNX\checkpoint.005_quant.onnx"
        )
        == "models/recognition/latin_printed_v2.onnx"
    )
    assert (
        resource_filename(r"C:\build\Model_Edge\Character\CJKPrinted\char2ind.txt")
        == "data/recognition/cjk_printed/alphabet.txt"
    )
    for checkpoint in ("checkpoint.001.onnx", "checkpoint.999.onnx"):
        assert (
            resource_filename("Model_Edge/Detector/Universal/ONNX/" + checkpoint)
            == "models/detection/universal.onnx"
        )
