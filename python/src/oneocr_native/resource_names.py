"""Stable semantic resource paths, independent of archive order/checkpoints."""

import re
from pathlib import PurePosixPath


def _slug(value: str) -> str:
    value = re.sub(r"([A-Z]+)([A-Z][a-z])", r"\1_\2", value)
    value = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", value)
    return re.sub(r"[^a-z0-9._-]+", "_", value.lower()).strip("_.-") or "resource"


def resource_filename(original: str) -> str:
    normalized = original.replace("\\", "/")
    parts = normalized.split("/")
    model = normalized.lower().endswith(".onnx")
    for index, part in enumerate(parts):
        if part.lower() == "model_edge" and index + 1 < len(parts):
            parts = parts[index + 1 :]
            break
    if len(parts) >= 2:
        role, profile, file = parts[0].lower(), _slug(parts[1]), parts[-1].lower()
        if model:
            category = {
                "detector": "detection",
                "character": "recognition",
                "rejection": "rejection",
                "confidence": "confidence",
                "linelayout": "layout",
            }.get(role)
            if category:
                return f"models/{category}/{profile}.onnx"
            if role == "auxmltcls":
                return "models/classification/script_orientation.onnx"
        else:
            if role == "character":
                name = {
                    "char2ind.txt": "alphabet.txt",
                    "char2inschar.txt": "character_mapping.txt",
                    "composite_chars_map": "composite_characters.txt",
                    "rnn.info": "rnn.info",
                }.get(file, _slug(file))
                return f"data/recognition/{profile}/{name}"
            if role == "detector" and file == "checkbox_cal.txt":
                return f"data/detection/{profile}/checkbox_calibration.txt"
            if role == "auxmltcls" and file == "handwritten_calibration_map.txt":
                return "data/classification/handwriting_calibration.txt"
            if role == "enums":
                return f"data/enums/{_slug(file)}"
    return ("models/" if model else "data/") + _slug(PurePosixPath(normalized).name)
