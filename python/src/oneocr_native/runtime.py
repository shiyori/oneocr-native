from __future__ import annotations

from pathlib import Path

import onnxruntime as ort

from .errors import UnsupportedModelError


def session(model: str | Path | bytes, threads: int = 1) -> ort.InferenceSession:
    options = ort.SessionOptions()
    options.intra_op_num_threads = threads
    options.inter_op_num_threads = 1
    options.log_severity_level = 3
    options.add_session_config_entry("session.intra_op.allow_spinning", "0")
    options.add_session_config_entry("session.inter_op.allow_spinning", "0")
    try:
        return ort.InferenceSession(
            str(model) if isinstance(model, Path) else model,
            sess_options=options,
            providers=["CPUExecutionProvider"],
        )
    except Exception as exc:
        raise UnsupportedModelError(f"ONNX Runtime could not load this model: {exc}") from exc
