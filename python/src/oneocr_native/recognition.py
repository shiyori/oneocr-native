from __future__ import annotations

import math
import unicodedata
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

import cv2
import numpy as np

if TYPE_CHECKING:
    import onnxruntime as ort

from .bidi import visual_to_logical
from .config import CharacterModel
from .errors import ModelFormatError, UnsupportedModelError

CONFIDENCE_METHOD = "ctc_token_geometric_mean"


@dataclass(frozen=True)
class Recognition:
    text: str = ""
    log_probability: float = 0.0
    tokens: int = 0

    @property
    def confidence(self) -> float | None:
        if not self.text or not self.tokens:
            return None
        return math.exp(self.log_probability / self.tokens)


def normalize_line(rgb: np.ndarray, *, stride: int = 4) -> np.ndarray:
    height, width = rgb.shape[:2]
    scaled_width = max(8, round(width * 60 / height))
    # The convolutional encoder accepts variable width. White side padding
    # protects edge characters; output frames are decoded, not rescaled boxes.
    resized = cv2.resize(rgb, (scaled_width, 60), interpolation=cv2.INTER_LINEAR)
    left = 16
    right = 16 + (-(scaled_width + 32) % stride)
    padded = cv2.copyMakeBorder(
        resized, 0, 0, left, right, cv2.BORDER_CONSTANT, value=(255, 255, 255)
    )
    return (padded.astype(np.float32) / 255).transpose(2, 0, 1)[None].copy()


class Alphabet:
    def __init__(self, alphabet_path: Path | bytes, composite_path: Path | bytes | None = None):
        self.characters: dict[int, str] = {}
        for line in (
            alphabet_path.decode("utf-8")
            if isinstance(alphabet_path, bytes)
            else alphabet_path.read_text(encoding="utf-8")
        ).splitlines():
            try:
                char, value = line.rsplit(" ", 1)
                index = int(value)
            except ValueError as exc:
                raise ModelFormatError("invalid character-index dictionary") from exc
            if index < 0 or index in self.characters:
                raise ModelFormatError("duplicate or negative character index")
            self.characters[index] = char
        blanks = [idx for idx, char in self.characters.items() if char == "<blank>"]
        if len(blanks) != 1 or set(self.characters) != set(range(len(self.characters))):
            raise ModelFormatError(
                "character dictionary must be contiguous with one explicit blank"
            )
        self.blank = blanks[0]
        self.composites: dict[str, str] = {}
        if composite_path:
            for line in (
                composite_path.decode("utf-8")
                if isinstance(composite_path, bytes)
                else composite_path.read_text(encoding="utf-8")
            ).splitlines():
                if not line or line.startswith("//"):
                    continue
                parts = line.split(" ", 1)
                if len(parts) != 2:
                    raise ModelFormatError("invalid composite character dictionary")
                self.composites[parts[0]] = parts[1]

    def decode(self, log_probabilities: np.ndarray, *, script: str = "Latin") -> tuple[str, float]:
        result = self.decode_scored(log_probabilities, script=script)
        return result.text, result.confidence if result.confidence is not None else 0.0

    def decode_scored(self, log_probabilities: np.ndarray, *, script: str = "Latin") -> Recognition:
        if (
            log_probabilities.ndim != 2
            or log_probabilities.shape[1] != len(self.characters)
            or not np.isfinite(log_probabilities).all()
        ):
            raise ModelFormatError("recognizer output does not match the character dictionary")
        ids = log_probabilities.argmax(axis=1)
        characters = []
        emissions = []
        previous = -1
        for frame, idx in enumerate(ids):
            idx = int(idx)
            if idx != previous and idx != self.blank:
                token = self.characters[idx]
                if token in ("<space>", "<trash_as_space>"):
                    token = " "
                elif token.startswith("<") and token.endswith(">"):
                    raise UnsupportedModelError(f"unsupported dictionary token: {token}")
                token = self.composites.get(token, token)
                characters.append(token)
                if token:
                    emissions.append((frame, idx, token))
            previous = idx
        text = "".join(characters).strip()
        if script in ("Arabic", "Hebrew"):
            text = visual_to_logical(text)
        text = unicodedata.normalize("NFC", text)
        if not text:
            return Recognition()
        while emissions and not emissions[0][2].strip():
            emissions.pop(0)
        while emissions and not emissions[-1][2].strip():
            emissions.pop()
        log_probability = 0.0
        for frame, idx, _ in emissions:
            row = log_probabilities[frame].astype(np.float64)
            maximum = float(row.max())
            log_probability += (
                float(row[idx]) - maximum - math.log(float(np.exp(row - maximum).sum()))
            )
        return Recognition(text, log_probability, len(emissions))


class Recognizer:
    def __init__(self, model: ort.InferenceSession, config: CharacterModel, alphabet: Alphabet):
        self.model = model
        self.config = config
        self.alphabet = alphabet
        output = model.get_outputs()[0]
        if output.name != "logsoftmax" or output.shape[-1] != len(alphabet.characters):
            raise UnsupportedModelError("recognizer graph and alphabet are incompatible")

    def run(self, rgb: np.ndarray) -> tuple[str, float]:
        result = self.run_scored(rgb)
        return result.text, result.confidence if result.confidence is not None else 0.0

    def run_scored(self, rgb: np.ndarray) -> Recognition:
        data = normalize_line(rgb, stride=self.config.pixels_per_frame)
        if data.shape[-1] > 8192:
            raise UnsupportedModelError("text line exceeds 8192 normalized pixels; split the image")
        output = self.model.run(
            ["logsoftmax"],
            {
                "data": data,
                "seq_lengths": np.array(
                    [data.shape[-1] // self.config.pixels_per_frame], dtype=np.int32
                ),
            },
        )[0]
        return self.alphabet.decode_scored(output[:, 0, :], script=self.config.script)
