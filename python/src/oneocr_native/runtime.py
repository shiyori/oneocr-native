from __future__ import annotations

import importlib
from pathlib import Path
from typing import TYPE_CHECKING

from .errors import UnsupportedModelError

if TYPE_CHECKING:
    import onnxruntime as ort


def get_runtime():
    try:
        runtime = importlib.import_module("onnxruntime")
    except ImportError as exc:
        raise UnsupportedModelError(
            "ONNX Runtime is unavailable; run python -m oneocr_native install"
        ) from exc
    version = runtime.__version__.split(".")
    try:
        compatible = int(version[0]) == 1 and int(version[1]) >= 26
    except (IndexError, ValueError):
        compatible = False
    if not compatible:
        raise UnsupportedModelError(
            f"existing ONNX Runtime {runtime.__version__} is incompatible; requires 1.26 or newer 1.x"
        )
    return runtime


def session(model: str | Path | bytes, threads: int = 1) -> ort.InferenceSession:
    runtime = get_runtime()
    options = runtime.SessionOptions()
    options.intra_op_num_threads = threads
    options.inter_op_num_threads = 1
    options.log_severity_level = 3
    options.add_session_config_entry("session.intra_op.allow_spinning", "0")
    options.add_session_config_entry("session.inter_op.allow_spinning", "0")
    try:
        return runtime.InferenceSession(
            str(model) if isinstance(model, Path) else model,
            sess_options=options,
            providers=["CPUExecutionProvider"],
        )
    except Exception as exc:
        raise UnsupportedModelError(f"ONNX Runtime could not load this model: {exc}") from exc
