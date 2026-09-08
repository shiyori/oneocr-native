"""Portable, complete resources for native SDKs and custom ONNX integrations."""

from __future__ import annotations

import json
import shutil
import tempfile
import zipfile
from dataclasses import asdict
from hashlib import sha256
from pathlib import Path, PurePosixPath

import onnx

from .cache import PreparedModel, prepare
from .config import CLASSIFIER_SCRIPTS, CharacterModel, PipelineConfig
from .errors import ModelFormatError
from .resource_names import resource_filename

SCHEMA = "oneocr.bundle.v1"
PIPELINE_SPEC = {
    "profile": "relationrcnn-seglink-dblstm-v1",
    "detector": {
        "color": "RGB",
        "dtype": "float32",
        "layout": "NCHW",
        "range": [0, 255],
        "embedded_normalization": True,
        "padding_multiple": 32,
        "padding_rgb": [255, 255, 255],
        "levels": [2, 3, 4],
        "stride": "2**level",
        "center_offset": "(stride-1)/2",
        "regression_scale": "8*stride-1",
        "link_threshold": 0.8,
        "nms_iou": 0.2,
        "neighbors_yx": [[-1, -1], [-1, 0], [-1, 1], [0, -1], [0, 1], [1, -1], [1, 0], [1, 1]],
        "connectivity": "either directional link >= threshold",
    },
    "classifier": {
        "scripts": list(CLASSIFIER_SCRIPTS),
        "flip": "rotate 180 degrees if flip_score < 0",
    },
    "recognizer": {
        "color": "RGB",
        "dtype": "float32",
        "layout": "NCHW",
        "height": 60,
        "range": [0, 1],
        "side_padding": 16,
        "width_alignment": "pixels_per_frame",
        "sequence_length": "padded_width/pixels_per_frame",
        "decoder": "greedy CTC",
        "blank": "index explicitly labelled <blank> in alphabet; never assume 0",
        "unicode": "expand composite maps; visual-to-logical RTL; NFC",
    },
    "limitations": [
        "Independent final quad fitting, line normalization and reading order",
        "Rejection/calibration models exported but not used by the reference pipeline",
        "confidence and words are null; natural vertical CJK and handwriting not validated",
    ],
}


def tensor_interface(data: bytes) -> dict:
    model = onnx.load_model_from_string(data)
    initializers = {tensor.name for tensor in model.graph.initializer}

    def info(value):
        tensor = value.type.tensor_type
        shape = [
            dim.dim_value if dim.HasField("dim_value") else dim.dim_param or None
            for dim in tensor.shape.dim
        ]
        return {
            "name": value.name,
            "dtype": onnx.TensorProto.DataType.Name(tensor.elem_type),
            "shape": shape,
        }

    return {
        "inputs": [info(value) for value in model.graph.input if value.name not in initializers],
        "outputs": [info(value) for value in model.graph.output],
        "opsets": {item.domain or "ai.onnx": item.version for item in model.opset_import},
        "operators": sorted(
            {f"{node.domain or 'ai.onnx'}::{node.op_type}" for node in model.graph.node}
        ),
    }


def export_bundle(
    model_path: str | Path,
    directory: str | Path,
    *,
    archive: str | Path | None = None,
    cache_dir: str | Path | None = None,
) -> Path:
    """Export all resources. Does not overwrite an existing developer bundle."""
    prepared = prepare(model_path, cache_dir)
    target = Path(directory).absolute()
    if target.exists():
        raise ModelFormatError(f"bundle destination already exists: {target}")
    target.parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix=".bundle-", dir=target.parent))
    try:
        resources = []
        names = {}
        used_names = set()
        for resource in prepared.manifest["resources"]:
            data = (prepared.directory / resource["file"]).read_bytes()
            is_model = resource["file"].endswith(".onnx")
            filename = resource_filename(resource["name"])
            if filename in used_names:
                raise ModelFormatError(f"colliding standard resource name: {filename}")
            used_names.add(filename)
            path = stage / filename
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
            names[resource["name"]] = filename
            item = {
                "id": resource["index"],
                "file": filename,
                "original_name": resource["name"],
                "bytes": len(data),
                "sha256": sha256(data).hexdigest(),
                "kind": "onnx" if is_model else "data",
            }
            if is_model:
                item["interface"] = tensor_interface(data)
            resources.append(item)
        config = asdict(prepared.config)
        for key in ("detector_path", "classifier_path"):
            config[key] = names[config[key]]
        for character in config["characters"]:
            for key in (
                "model_path",
                "alphabet_path",
                "physical_map_path",
                "composite_path",
                "prior_path",
            ):
                character[key] = names[character[key]] if character[key] else ""
        raw = (prepared.directory / "config.pb").read_bytes()
        (stage / "config.pb").write_bytes(raw)
        manifest = {
            "schema": SCHEMA,
            "source_sha256": prepared.manifest["source_sha256"],
            "runtime": {
                "name": "onnxruntime",
                "tested_version": "1.29.0",
                "provider": "CPUExecutionProvider",
                "require_contrib_ops": True,
            },
            "config": {"file": "config.pb", "bytes": len(raw), "sha256": sha256(raw).hexdigest()},
            "pipeline": config,
            "resources": resources,
        }
        (stage / "bundle.json").write_text(
            json.dumps(manifest, ensure_ascii=False, indent=2) + "\n"
        )
        (stage / "pipeline.spec.json").write_text(
            json.dumps(PIPELINE_SPEC, ensure_ascii=False, indent=2) + "\n"
        )
        load_bundle(stage)
        stage.rename(target)
    finally:
        if stage.exists():
            shutil.rmtree(stage)
    if archive:
        destination = Path(archive)
        destination.parent.mkdir(parents=True, exist_ok=True)
        with zipfile.ZipFile(
            destination, "x", compression=zipfile.ZIP_DEFLATED, compresslevel=6
        ) as zipped:
            for path in sorted(target.rglob("*")):
                if path.is_file():
                    zipped.write(path, path.relative_to(target).as_posix())
    return target


def _checked_file(root: Path, entry: dict) -> Path:
    filename = entry["file"]
    if not isinstance(filename, str) or "\\" in filename or ":" in filename or "\x00" in filename:
        raise ModelFormatError("bundle: non-portable resource path")
    relative = PurePosixPath(filename)
    if relative.is_absolute() or ".." in relative.parts or not relative.parts:
        raise ModelFormatError("bundle: resource escapes its directory")
    path = root / relative
    if not path.resolve().is_relative_to(root.resolve()) or path.is_symlink():
        raise ModelFormatError("bundle: external or symlinked resource")
    if (
        path.stat().st_size != entry["bytes"]
        or sha256(path.read_bytes()).hexdigest() != entry["sha256"]
    ):
        raise ModelFormatError(f"bundle: resource integrity mismatch: {filename}")
    return path


def load_bundle(directory: str | Path) -> PreparedModel:
    root = Path(directory)
    try:
        manifest = json.loads((root / "bundle.json").read_text())
        if manifest["schema"] != SCHEMA:
            raise ModelFormatError("unsupported bundle schema")
        _checked_file(root, manifest["config"])
        files = set()
        for resource in manifest["resources"]:
            _checked_file(root, resource)
            if resource["file"] in files:
                raise ModelFormatError("bundle: duplicate resource")
            files.add(resource["file"])
        data = manifest["pipeline"]
        config = PipelineConfig(
            data["detector_path"],
            data["classifier_path"],
            tuple(CharacterModel(**char) for char in data["characters"]),
            data["segment_threshold"],
            {int(k): v for k, v in data["line_thresholds"].items()},
        )
        if not config.required_paths() <= files:
            raise ModelFormatError("bundle: missing pipeline dependency")
        # Adapter for the existing runtime's resource resolver. Windows names
        # remain metadata only; the portable pipeline uses relative paths.
        adapted = dict(manifest)
        adapted["resources"] = [
            dict(resource, name=resource["file"]) for resource in manifest["resources"]
        ]
        return PreparedModel(root, adapted, config)
    except (KeyError, TypeError, ValueError, OSError) as exc:
        if isinstance(exc, ModelFormatError):
            raise
        raise ModelFormatError(f"invalid bundle: {exc}") from exc
