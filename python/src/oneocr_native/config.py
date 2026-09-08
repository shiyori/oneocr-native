"""Small, bounded protobuf wire reader for the fields used by the OCR profile.

Field numbers were checked against oneocr.proto descriptors embedded in the
supplied DLL. Unknown fields remain in the original cached config.pb; they are
not interpreted as code, paths to open, or runtime instructions.
"""

from __future__ import annotations

import math
import struct
from dataclasses import dataclass

from .errors import ModelFormatError, UnsupportedModelError


def fields(data: bytes) -> dict[int, list[int | bytes | float]]:
    if not isinstance(data, bytes):
        raise ModelFormatError("configuration: expected a length-delimited message")
    pos = 0
    result: dict[int, list[int | bytes | float]] = {}

    def varint() -> int:
        nonlocal pos
        value = 0
        for shift in range(0, 70, 7):
            if pos >= len(data):
                raise ModelFormatError("configuration: truncated varint")
            byte = data[pos]
            pos += 1
            if shift == 63 and byte > 1:
                raise ModelFormatError("configuration: varint overflow")
            value |= (byte & 127) << shift
            if byte < 128:
                return value
        raise ModelFormatError("configuration: invalid varint")

    def take(n: int) -> bytes:
        nonlocal pos
        if n > len(data) - pos:
            raise ModelFormatError("configuration: truncated field")
        value = data[pos : pos + n]
        pos += n
        return value

    while pos < len(data):
        tag = varint()
        number, wire = tag >> 3, tag & 7
        if not 0 < number < 2**29:
            raise ModelFormatError("configuration: invalid field number")
        if wire == 0:
            value: int | bytes | float = varint()
        elif wire == 1:
            value = struct.unpack("<d", take(8))[0]
        elif wire == 2:
            value = take(varint())
        elif wire == 5:
            value = struct.unpack("<f", take(4))[0]
        else:
            raise ModelFormatError(f"configuration: unsupported wire type {wire}")
        result.setdefault(number, []).append(value)
    return result


def one(message: dict, number: int, default=None):
    values = message.get(number, [])
    if len(values) > 1:
        raise ModelFormatError(f"configuration: duplicate singular field {number}")
    return values[0] if values else default


def text(message: dict, number: int, default: str = "") -> str:
    value = one(message, number, default.encode())
    if not isinstance(value, bytes):
        raise ModelFormatError(f"configuration: field {number} is not text")
    try:
        return value.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ModelFormatError("configuration: invalid UTF-8") from exc


def nested(message: dict, number: int) -> dict:
    value = one(message, number, b"")
    if not isinstance(value, bytes):
        raise ModelFormatError(f"configuration: field {number} is not a message")
    return fields(value)


def probability(message: dict, number: int, default: float) -> float:
    value = one(message, number, default)
    if not isinstance(value, (int, float)) or not math.isfinite(value) or not 0 < value <= 1:
        raise ModelFormatError(f"configuration: field {number} must be a finite probability")
    return float(value)


def model_path(message: dict, pack_field: int) -> str:
    return text(nested(nested(message, pack_field), 1), 1)


SCRIPTS = ("Latin", "CJK", "Cyrillic", "Arabic", "Devanagari", "Greek", "Thai", "Hebrew", "Tamil")
# Classifier output channels; channel 0 is non-text, then the script enum order.
CLASSIFIER_SCRIPTS = (
    None,
    "CJK",
    "Latin",
    "Cyrillic",
    "Arabic",
    "Devanagari",
    "Greek",
    "Thai",
    "Hebrew",
    "Tamil",
)


@dataclass(frozen=True)
class CharacterModel:
    name: str
    script: str
    model_path: str
    alphabet_path: str
    physical_map_path: str
    composite_path: str
    prior_path: str
    pixels_per_frame: int


@dataclass(frozen=True)
class PipelineConfig:
    detector_path: str
    classifier_path: str
    characters: tuple[CharacterModel, ...]
    segment_threshold: float
    line_thresholds: dict[int, float]

    @classmethod
    def parse(cls, data: bytes) -> PipelineConfig:
        top = fields(data)
        detector = nested(top, 1)
        classifier = nested(top, 20)
        if one(detector, 2, 0) != 0 or one(classifier, 12, 0) != 0:
            raise UnsupportedModelError(
                "only the dynamic RelationRCNN + Script/HWPC/Flip profile is implemented"
            )
        characters = []
        for raw in top.get(3, []):
            if not isinstance(raw, bytes):
                raise ModelFormatError("configuration: invalid character model")
            char = fields(raw)
            kind = one(char, 2, 0)
            stride = one(char, 7, 0)
            if not isinstance(kind, int) or kind not in range(len(SCRIPTS)) or stride not in (4, 8):
                raise UnsupportedModelError("unsupported character model type or frame stride")
            characters.append(
                CharacterModel(
                    text(char, 1),
                    SCRIPTS[kind],
                    model_path(char, 3),
                    text(char, 5),
                    text(char, 6),
                    text(char, 12),
                    text(char, 4),
                    stride,
                )
            )
        if not characters or len({x.script for x in characters}) != len(characters):
            raise UnsupportedModelError("expected one character model per available script")
        thresholds = {}
        for raw in detector.get(9, []):
            item = fields(raw)
            name = text(item, 1)
            if name in ("P2", "P3", "P4"):
                thresholds[int(name[1])] = probability(item, 2, 0.8)
        if set(thresholds) != {2, 3, 4}:
            raise UnsupportedModelError("detector must define P2/P3/P4 line thresholds")
        result = cls(
            model_path(detector, 3),
            model_path(classifier, 2),
            tuple(characters),
            probability(detector, 8, 0.7),
            thresholds,
        )
        if not result.detector_path or not result.classifier_path:
            raise ModelFormatError("configuration: required detector/classifier is missing")
        if not all(
            math.isfinite(x) and 0 < x <= 1
            for x in [result.segment_threshold, *thresholds.values()]
        ):
            raise ModelFormatError(
                "configuration: detector thresholds must be finite probabilities"
            )
        return result

    def required_paths(self) -> set[str]:
        paths = {self.detector_path, self.classifier_path}
        for char in self.characters:
            paths.update(
                p
                for p in (
                    char.model_path,
                    char.alphabet_path,
                    char.physical_map_path,
                    char.composite_path,
                    char.prior_path,
                )
                if p
            )
        return paths
