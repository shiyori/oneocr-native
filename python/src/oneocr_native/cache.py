from __future__ import annotations

import json
import os
import shutil
import tempfile
from dataclasses import asdict, dataclass
from hashlib import sha256
from pathlib import Path

import numpy as np
import onnx
import onnxruntime as ort
from filelock import FileLock

from .config import PipelineConfig
from .container import ModelContainer
from .errors import ModelFormatError, UnsupportedModelError
from .runtime import session

CONVERTER_VERSION = "cbc-1"


@dataclass(frozen=True)
class PreparedModel:
    directory: Path
    manifest: dict
    config: PipelineConfig
    cache_hit: bool = False

    def read(self, original_name: str) -> bytes:
        return self.path(original_name).read_bytes()

    def close(self):
        """Directory-backed sources hold no open descriptor."""

    def path(self, original_name: str) -> Path:
        for resource in self.manifest["resources"]:
            if resource["name"] == original_name:
                return self.directory / resource["file"]
        raise ModelFormatError(f"missing model resource: {original_name}")


def _audit_model(data: bytes) -> dict:
    model = onnx.load_model_from_string(data)
    if any(t.data_location == onnx.TensorProto.EXTERNAL for t in model.graph.initializer):
        raise UnsupportedModelError("external ONNX tensor files are not supported")
    onnx.checker.check_model(model)
    sess = session(data)
    inputs = {i.name: i for i in sess.get_inputs()}
    feeds = {}
    for name, item in inputs.items():
        if name == "im_info":
            feeds[name] = np.array([[64, 64, 1]], dtype=np.float32)
        elif name == "seq_lengths":
            # A short valid sequence for every supplied DBLSTM recognizer.
            feeds[name] = np.array([1], dtype=np.int32)
        elif name == "data" and item.type == "tensor(float)":
            dims = [
                dim if isinstance(dim, int) else (64 if axis > 1 else 1)
                for axis, dim in enumerate(item.shape)
            ]
            if not dims or any(dim <= 0 or dim > 4096 for dim in dims):
                raise UnsupportedModelError(f"unsupported ONNX input shape: {item.shape}")
            feeds[name] = np.zeros(dims, dtype=np.float32)
        else:
            raise UnsupportedModelError(
                f"unimplemented model input: {name} {item.type} {item.shape}"
            )
    outputs = sess.run(None, feeds)
    if not all(np.isfinite(output).all() for output in outputs):
        raise UnsupportedModelError("model smoke inference produced nonfinite values")
    return {
        "inputs": [{"name": i.name, "shape": i.shape, "type": i.type} for i in inputs.values()],
        "outputs": [
            {"name": item.name, "shape": list(value.shape)}
            for item, value in zip(sess.get_outputs(), outputs)
        ],
        "operators": sorted(
            {f"{node.domain or 'ai.onnx'}::{node.op_type}" for node in model.graph.node}
        ),
        "cpu_load": "passed",
        "cpu_smoke_inference": "passed",
    }


def _read_valid_cache(directory: Path, container: ModelContainer) -> PreparedModel | None:
    try:
        manifest = json.loads((directory / "manifest.json").read_text())
        if (
            manifest["source_sha256"] != container.source_hash
            or manifest["converter_version"] != CONVERTER_VERSION
            or manifest["onnxruntime_version"] != ort.__version__
            or manifest["config_sha256"] != sha256(container.config).hexdigest()
            or len(manifest["resources"]) != len(container.resources)
        ):
            return None
        config_data = (directory / "config.pb").read_bytes()
        if config_data != container.config:
            return None
        for entry, resource in zip(manifest["resources"], container.resources):
            if (
                entry["file"] != resource.filename
                or entry["name"] != resource.name
                or entry["sha256"] != resource.digest
            ):
                return None
            path = directory / resource.filename
            if path.is_symlink() or sha256(path.read_bytes()).hexdigest() != resource.digest:
                return None
        return PreparedModel(directory, manifest, PipelineConfig.parse(config_data), cache_hit=True)
    except (OSError, ValueError, KeyError, TypeError, ModelFormatError):
        return None


def prepare(model_path: str | Path, cache_dir: str | Path | None = None) -> PreparedModel:
    container = ModelContainer.load(model_path)
    config = PipelineConfig.parse(container.config)
    missing = config.required_paths() - {entry.name for entry in container.resources}
    if missing:
        raise ModelFormatError(
            f"model configuration references missing resources: {sorted(missing)}"
        )
    root = Path(cache_dir).expanduser() if cache_dir else Path.home() / ".cache" / "oneocr-native"
    directory = root / container.source_hash / CONVERTER_VERSION
    directory.parent.mkdir(parents=True, exist_ok=True)
    with FileLock(directory.parent / ".prepare.lock"):
        return _prepare_locked(container, config, directory)


def _prepare_locked(
    container: ModelContainer, config: PipelineConfig, directory: Path
) -> PreparedModel:
    if directory.is_symlink():
        raise ModelFormatError("cache destination must not be a symlink")
    cached = _read_valid_cache(directory, container)
    if cached:
        return cached
    staging = Path(tempfile.mkdtemp(prefix=".prepare-", dir=directory.parent))
    try:
        resources = []
        for resource in container.resources:
            entry = {
                "index": resource.index,
                "name": resource.name,
                "file": resource.filename,
                "bytes": len(resource.data),
                "sha256": resource.digest,
                "offset": resource.offset,
                "stored_size": resource.stored_size,
            }
            if resource.filename.endswith(".onnx"):
                try:
                    entry["validation"] = _audit_model(resource.data)
                except Exception as exc:
                    raise UnsupportedModelError(
                        f"resource {resource.index} ({resource.name}): {exc}"
                    ) from exc
            (staging / resource.filename).write_bytes(resource.data)
            resources.append(entry)
        (staging / "config.pb").write_bytes(container.config)
        manifest = {
            "format": "OneModel AES-256-CBC",
            "converter_version": CONVERTER_VERSION,
            "source_sha256": container.source_hash,
            "source_bytes": container.source_size,
            "config_sha256": sha256(container.config).hexdigest(),
            "onnxruntime_version": ort.__version__,
            "provider": "CPUExecutionProvider",
            "pipeline": asdict(config),
            "resources": resources,
        }
        (staging / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2))
        # Only this content-addressed, application-owned cache is replaced.
        # Concurrent preparation can reuse another process's completed result.
        existing = _read_valid_cache(directory, container)
        if existing:
            return existing
        if directory.is_symlink():
            raise ModelFormatError("cache destination must not be a symlink")
        if directory.exists():
            shutil.rmtree(directory)
        try:
            os.rename(staging, directory)
        except OSError:
            existing = _read_valid_cache(directory, container)
            if existing:
                return existing
            raise
        return PreparedModel(directory, manifest, config)
    finally:
        if staging.exists():
            shutil.rmtree(staging)
