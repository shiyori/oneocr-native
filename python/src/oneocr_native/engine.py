from __future__ import annotations

from dataclasses import asdict, dataclass, field
from pathlib import Path
from threading import RLock
from time import perf_counter

import cv2
import numpy as np
from PIL import Image, ImageOps

from .cache import prepare
from .config import CLASSIFIER_SCRIPTS
from .detection import Detector
from .errors import OneOcrError, UnsupportedModelError
from .geometry import reading_order, rectify
from .recognition import Alphabet, Recognizer, normalize_line
from .runtime import session


@dataclass(frozen=True)
class OcrLine:
    text: str
    quad: list[list[float]]
    script: str
    confidence: float | None = None
    words: list[dict] | None = None


@dataclass(frozen=True)
class OcrResult:
    text: str
    lines: list[OcrLine]
    width: int
    height: int
    elapsed_seconds: float
    model_sha256: str
    warnings: list[str] = field(default_factory=list)

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass(frozen=True)
class DetectionRegion:
    quad: list[list[float]]
    score: float
    vertical: bool


@dataclass(frozen=True)
class DetectionResult:
    regions: list[DetectionRegion]
    width: int
    height: int
    elapsed_seconds: float
    model_sha256: str

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass(frozen=True)
class LineResult:
    text: str
    script: str
    rotated_180: bool
    elapsed_seconds: float
    model_sha256: str

    def to_dict(self) -> dict:
        return asdict(self)


def _load_image(image: str | Path | Image.Image) -> tuple[Image.Image, np.ndarray]:
    if isinstance(image, (str, Path)):
        with Image.open(image) as source:
            pil = ImageOps.exif_transpose(source).copy()
    elif isinstance(image, Image.Image):
        pil = ImageOps.exif_transpose(image).copy()
    else:
        raise TypeError("image must be a path or PIL.Image.Image")
    if pil.width < 2 or pil.height < 2 or pil.width * pil.height > 40_000_000:
        raise OneOcrError("image must be at least 2×2 and at most 40 megapixels")
    if "A" in pil.getbands() or "transparency" in pil.info:
        rgba = pil.convert("RGBA")
        pil = Image.alpha_composite(Image.new("RGBA", rgba.size, "white"), rgba)
    return pil, np.asarray(pil.convert("RGB"))


class OneOcrEngine:
    """Experimental OneOCR inference on the local ONNX Runtime CPU provider.

    ONEOCRPK resources load directly from one verified file. Original OneModel
    input is prepared into a content-addressed cache. No project Go library or
    original Windows DLL is loaded. Recognition models load lazily; close the
    engine or use a context manager to release owned resources.
    """

    def __init__(
        self,
        model_path: str | Path,
        *,
        cache_dir: str | Path | None = None,
        max_side: int = 1600,
        threads: int = 2,
    ):
        if not 128 <= max_side <= 4096:
            raise ValueError("max_side must be between 128 and 4096")
        if not 1 <= threads <= 16:
            raise ValueError("threads must be between 1 and 16")
        from .bundle import load_bundle
        from .ocrpack import MAGIC, PackageSource

        path = Path(model_path)
        if path.is_dir():
            self.prepared = load_bundle(path)
        else:
            with path.open("rb") as stream:
                is_package = stream.read(8) == MAGIC
            self.prepared = PackageSource(path) if is_package else prepare(path, cache_dir)
        self._initialize(max_side, threads)

    @classmethod
    def from_bundle(cls, directory: str | Path, *, max_side: int = 1600, threads: int = 2):
        """Load a portable developer bundle, without the original .onemodel."""
        from .bundle import load_bundle

        if not 128 <= max_side <= 4096 or not 1 <= threads <= 16:
            raise ValueError("invalid max_side or threads")
        instance = cls.__new__(cls)
        instance.prepared = load_bundle(directory)
        instance._initialize(max_side, threads)
        return instance

    @classmethod
    def from_package(cls, filename: str | Path, *, max_side: int = 1600, threads: int = 2):
        """Load a verified .ocrpack without extracting files or using Go libraries."""
        from .ocrpack import PackageSource

        if not 128 <= max_side <= 4096 or not 1 <= threads <= 16:
            raise ValueError("invalid max_side or threads")
        instance = cls.__new__(cls)
        instance.prepared = PackageSource(filename)
        instance._initialize(max_side, threads)
        return instance

    def close(self):
        """Idempotently release owned session references and the package descriptor."""
        with self._lock:
            if self._closed:
                return
            self._closed = True
            self.recognizers.clear()
            self.detector = None
            self.classifier = None
            self.prepared.close()

    def __enter__(self):
        with self._lock:
            if self._closed:
                raise OneOcrError("engine is closed")
            return self

    def __exit__(self, *_):
        self.close()

    def _initialize(self, max_side: int, threads: int):
        self._lock = RLock()
        self._closed = False
        self.threads = threads
        self.recognizers: dict[str, Recognizer] = {}
        self.detector = None
        self.classifier = None
        self.characters = {char.script: char for char in self.prepared.config.characters}
        try:
            self.detector = Detector(
                session(self.prepared.read(self.prepared.config.detector_path), threads),
                self.prepared.config,
                max_side,
            )
            self.classifier = session(
                self.prepared.read(self.prepared.config.classifier_path), threads
            )
        except Exception:
            self.close()
            raise

    @property
    def available_scripts(self) -> tuple[str, ...]:
        """Model inventory; see validation report for tested script coverage."""
        return tuple(self.characters)

    def _recognizer(self, script: str) -> Recognizer:
        if script not in self.recognizers:
            char = self.characters[script]
            alphabet = Alphabet(
                self.prepared.read(char.alphabet_path),
                self.prepared.read(char.composite_path) if char.composite_path else None,
            )
            self.recognizers[script] = Recognizer(
                session(self.prepared.read(char.model_path), self.threads), char, alphabet
            )
        return self.recognizers[script]

    def _classify(self, crop: np.ndarray) -> tuple[str | None, float]:
        data = normalize_line(crop)
        if data.shape[-1] > 8192:
            raise UnsupportedModelError(
                "detected line is too long for classification; split the image"
            )
        values = self.classifier.run(["script_id_score", "flip_score"], {"data": data})
        scores = values[0].reshape(-1)
        if len(scores) != len(CLASSIFIER_SCRIPTS) or not np.isfinite(scores).all():
            raise UnsupportedModelError("unexpected script classifier output")
        return CLASSIFIER_SCRIPTS[int(scores.argmax())], float(values[1].reshape(-1)[0])

    def recognize(self, image: str | Path | Image.Image, *, script: str | None = None) -> OcrResult:
        """Recognize an image; optional script overrides automatic script selection.

        Coordinates refer to the EXIF-oriented input image, before detector
        resizing. Confidence is intentionally null: original calibration and
        rejection feature extraction have not been reimplemented.
        """
        with self._lock:
            if self._closed:
                raise OneOcrError("engine is closed")
            return self._recognize(image, script=script)

    def detect(self, image: str | Path | Image.Image) -> DetectionResult:
        """Detect proposals only. Scores are uncalibrated; region order is unspecified."""
        with self._lock:
            if self._closed:
                raise OneOcrError("engine is closed")
            start = perf_counter()
            pil, rgb = _load_image(image)
            detections = self.detector.run(rgb)
            if len(detections) > 1000:
                raise OneOcrError("more than 1000 detected regions; split the document image")
            return DetectionResult(
                [
                    DetectionRegion(d.quad.astype(float).tolist(), float(d.score), bool(d.vertical))
                    for d in detections
                ],
                pil.width,
                pil.height,
                perf_counter() - start,
                self.prepared.manifest["source_sha256"],
            )

    def recognize_line(
        self, image: str | Path | Image.Image, *, script: str | None = None
    ) -> LineResult:
        """Recognize a cropped horizontal line without detection.

        Empty script auto-classifies and corrects a 180-degree rotation. Explicit
        script skips classification and requires an upright line crop.
        """
        with self._lock:
            if self._closed:
                raise OneOcrError("engine is closed")
            start = perf_counter()
            _, crop = _load_image(image)
            rotated = False
            if not script:
                script, flip = self._classify(crop)
                if flip < 0:
                    crop = cv2.rotate(crop, cv2.ROTATE_180)
                    rotated = True
            text = ""
            if script:
                if script not in self.characters:
                    raise ValueError(f"unavailable script {script!r}")
                text, _ = self._recognizer(script).run(crop)
            return LineResult(
                text,
                script or "",
                rotated,
                perf_counter() - start,
                self.prepared.manifest["source_sha256"],
            )

    def _recognize(self, image: str | Path | Image.Image, *, script: str | None) -> OcrResult:
        start = perf_counter()
        if script and script not in self.characters:
            raise ValueError(f"unknown script {script!r}; available: {', '.join(self.characters)}")
        pil, rgb = _load_image(image)
        detections = self.detector.run(rgb)
        if len(detections) > 1000:
            raise OneOcrError("more than 1000 detected regions; split the document image")
        lines = []
        quads = []
        angles = []
        unavailable = set()
        for detection in detections:
            crop = rectify(rgb, detection.quad, vertical=detection.vertical)
            predicted, flip = self._classify(crop)
            selected = script or predicted
            if selected is None:
                continue
            if selected not in self.characters:
                unavailable.add(selected)
                continue
            if flip < 0:
                crop = cv2.rotate(crop, cv2.ROTATE_180)
            recognizer = self._recognizer(selected)
            text, _ = recognizer.run(crop)
            if not text:
                continue
            lines.append(OcrLine(text, detection.quad.astype(float).round(3).tolist(), selected))
            quads.append(detection.quad)
            q = detection.quad
            # Rectification rotates tall vertical crops counterclockwise;
            # a classifier flip changes the original-image reading vector.
            tall = np.linalg.norm(q[3] - q[0]) > np.linalg.norm(q[1] - q[0])
            vector = q[3] - q[0] if detection.vertical and tall else q[1] - q[0]
            angle = np.arctan2(vector[1], vector[0]) + (np.pi if flip < 0 else 0)
            angles.append(float(angle))
        if angles:
            direction = np.angle(np.sum(np.exp(1j * np.asarray(angles))))
            transform = np.array(
                [[np.cos(direction), -np.sin(direction)], [np.sin(direction), np.cos(direction)]]
            )
            layout_quads = [q @ transform for q in quads]
        else:
            layout_quads = quads
        rtl = sum(line.script in ("Arabic", "Hebrew") for line in lines) > len(lines) / 2
        ordered = [lines[i] for i in reading_order(layout_quads, rtl=rtl)]
        return OcrResult(
            "\n".join(line.text for line in ordered),
            ordered,
            pil.width,
            pil.height,
            perf_counter() - start,
            self.prepared.manifest["source_sha256"],
            [
                "Experimental segment grouping and reading order; original rejection/calibration are not applied."
            ]
            + [f"Skipped unavailable script: {name}" for name in sorted(unavailable)],
        )
