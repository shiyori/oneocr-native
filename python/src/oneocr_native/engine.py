from __future__ import annotations

import math
from dataclasses import asdict, dataclass, field
from pathlib import Path
from threading import RLock
from time import perf_counter

import cv2
import numpy as np
from PIL import Image, ImageOps

from .config import CLASSIFIER_SCRIPTS
from .detection import Detector
from .errors import OneOcrError, UnsupportedModelError
from .geometry import ordered_quad, reading_order, rectify
from .recognition import CONFIDENCE_METHOD, Alphabet, Recognition, Recognizer, normalize_line
from .routing import (
    ascii_digits,
    compact_quad,
    numeric_consensus,
    orientation_turns,
    page_orientation,
    quad_dimensions,
    rotate_quarter,
)
from .runtime import session


@dataclass(frozen=True)
class OcrLine:
    text: str
    quad: list[list[float]]
    script: str
    confidence: float | None = None
    words: list[dict] | None = None
    bbox: dict[str, float] | None = None
    detection_score: float = 0.0
    vertical: bool = False
    rotated_180: bool = False
    rotation_degrees: int = 0


@dataclass(frozen=True)
class OcrResult:
    text: str
    lines: list[OcrLine]
    width: int
    height: int
    elapsed_seconds: float
    model_sha256: str
    warnings: list[str] = field(default_factory=list)
    confidence: float | None = None
    confidence_method: str = CONFIDENCE_METHOD
    coordinate_space: str = "oriented_image"

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass(frozen=True)
class DetectionRegion:
    quad: list[list[float]]
    score: float
    vertical: bool
    bbox: dict[str, float] | None = None


@dataclass(frozen=True)
class DetectionResult:
    regions: list[DetectionRegion]
    width: int
    height: int
    elapsed_seconds: float
    model_sha256: str
    coordinate_space: str = "oriented_image"

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass(frozen=True)
class LineResult:
    text: str
    script: str
    rotated_180: bool
    elapsed_seconds: float
    model_sha256: str
    confidence: float | None = None
    confidence_method: str = CONFIDENCE_METHOD
    width: int = 0
    height: int = 0
    rotation_degrees: int = 0

    def to_dict(self) -> dict:
        return asdict(self)


@dataclass
class _RegionRecognition:
    line: OcrLine | None = None
    recognition: Recognition = field(default_factory=Recognition)
    script: str | None = None
    angle: float = 0.0
    anchor: bool = False


def _quad_bounds(quad: list[list[float]]) -> dict[str, float]:
    xs, ys = zip(*quad)
    return {"x": min(xs), "y": min(ys), "width": max(xs) - min(xs), "height": max(ys) - min(ys)}


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


@dataclass(frozen=True)
class EngineConfig:
    model_path: str | Path | None = None
    cache_dir: str | Path | None = None
    max_side: int = 1600
    threads: int = 2


class OneOcrEngine:
    """Synchronous offline OCR. Reuse an engine or manage it with a with block."""

    def __init__(self, config: EngineConfig | None = None):
        config = config or EngineConfig()
        if not isinstance(config, EngineConfig):
            raise TypeError("config must be EngineConfig")
        if not 128 <= config.max_side <= 4096:
            raise ValueError("max_side must be between 128 and 4096")
        if not 1 <= config.threads <= 16:
            raise ValueError("threads must be between 1 and 16")
        from .defaults import default_model_path
        from .ocrpack import MAGIC, PackageSource

        path = default_model_path() if config.model_path is None else Path(config.model_path)
        if path.is_dir():
            from .bundle import load_bundle

            self.prepared = load_bundle(path)
        else:
            with path.open("rb") as stream:
                is_package = stream.read(8) == MAGIC
            if is_package:
                self.prepared = PackageSource(path)
            else:
                from .cache import prepare

                self.prepared = prepare(path, config.cache_dir)
        self._initialize(config.max_side, config.threads)

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
        """Scripts provided by the active model."""
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
        flip = values[1].reshape(-1)
        if len(flip) != 1 or not np.isfinite(flip).all():
            raise UnsupportedModelError("unexpected flip classifier output")
        return CLASSIFIER_SCRIPTS[int(scores.argmax())], float(flip[0])

    def _unknown_numeral(self, crop: np.ndarray) -> tuple[Recognition, str | None]:
        if not {"Latin", "CJK"} <= self.characters.keys():
            return Recognition(), None
        latin = self._recognizer("Latin").run_scored(crop)
        if not ascii_digits(latin.text) or latin.confidence is None or latin.confidence < 0.90:
            return Recognition(), None
        cjk = self._recognizer("CJK").run_scored(crop)
        return (latin, "Latin") if numeric_consensus(latin, cjk) else (Recognition(), None)

    def _recognize_region(
        self, rgb: np.ndarray, detection, script: str | None, prior: float | None
    ) -> _RegionRecognition:
        compact = compact_quad(detection.quad)
        width, height = quad_dimensions(detection.quad)
        use_vertical = detection.vertical and (
            not compact or (prior is None and max(width, height) > 1.5 * min(width, height))
        )
        crop = rectify(rgb, detection.quad, vertical=use_vertical)
        quarter = int(use_vertical and max(2, round(height)) > max(2, round(width)))
        if compact and prior is not None:
            quarter = orientation_turns(detection.quad, prior)
            crop = rotate_quarter(crop, quarter)
        predicted, flip = self._classify(crop)
        selected = script or predicted
        if flip < 0 and (not compact or abs(flip) >= 2):
            crop = cv2.rotate(crop, cv2.ROTATE_180)
            quarter = (quarter + 2) % 4
        result = _RegionRecognition(script=selected)
        recognized = Recognition()
        if selected is None:
            if compact and detection.score >= 0.80:
                recognized, selected = self._unknown_numeral(crop)
        elif selected in self.characters:
            recognized = self._recognizer(selected).run_scored(crop)
        if not recognized.text:
            return result
        quad = detection.quad.astype(float).round(3).tolist()
        result.line = OcrLine(
            recognized.text,
            quad,
            selected,
            confidence=recognized.confidence,
            bbox=_quad_bounds(quad),
            detection_score=float(detection.score),
            vertical=bool(detection.vertical),
            rotated_180=quarter >= 2,
            rotation_degrees=(360 - quarter * 90) % 360,
        )
        result.recognition = recognized
        q = ordered_quad(detection.quad)
        edge = q[1] - q[0]
        result.angle = math.atan2(float(edge[1]), float(edge[0])) + quarter * math.pi / 2
        result.anchor = (
            not compact
            and abs(flip) >= 2
            and len(recognized.text) >= 3
            and recognized.confidence is not None
            and recognized.confidence >= 0.80
        )
        return result

    def recognize(self, image: str | Path | Image.Image, *, script: str | None = None) -> OcrResult:
        """Recognize an image; optional script overrides automatic script selection.

        Coordinates refer to the EXIF-oriented input image, before detector
        resizing. Confidence summarizes emitted CTC token probabilities; it is
        uncalibrated and is not an estimated probability of a correct result.
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
                    DetectionRegion(
                        d.quad.astype(float).tolist(),
                        float(d.score),
                        bool(d.vertical),
                        _quad_bounds(d.quad.astype(float).tolist()),
                    )
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
            pil, crop = _load_image(image)
            rotated = False
            compact = max(crop.shape[:2]) <= 2 * min(crop.shape[:2])
            if not script:
                script, flip = self._classify(crop)
                if flip < 0 and (not compact or abs(flip) >= 2):
                    crop = cv2.rotate(crop, cv2.ROTATE_180)
                    rotated = True
            recognized = Recognition()
            if script:
                if script not in self.characters:
                    raise ValueError(f"unavailable script {script!r}")
                recognized = self._recognizer(script).run_scored(crop)
            elif compact:
                recognized, script = self._unknown_numeral(crop)
            return LineResult(
                recognized.text,
                script or "",
                rotated,
                perf_counter() - start,
                self.prepared.manifest["source_sha256"],
                confidence=recognized.confidence,
                width=pil.width,
                height=pil.height,
                rotation_degrees=180 if rotated else 0,
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
        log_probability, tokens = 0.0, 0
        quads = []
        angles = []
        unavailable = set()
        regions = [_RegionRecognition() for _ in detections]
        for i, detection in enumerate(detections):
            if not compact_quad(detection.quad):
                regions[i] = self._recognize_region(rgb, detection, script, None)
        prior = page_orientation([(r.angle, len(r.recognition.text)) for r in regions if r.anchor])
        for i, detection in enumerate(detections):
            if compact_quad(detection.quad):
                regions[i] = self._recognize_region(rgb, detection, script, prior)
            region = regions[i]
            if region.line is None:
                if region.script and region.script not in self.characters:
                    unavailable.add(region.script)
                continue
            lines.append(region.line)
            log_probability += region.recognition.log_probability
            tokens += region.recognition.tokens
            quads.append(detection.quad)
            angles.append(region.angle)
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
            confidence=Recognition(
                "\n".join(line.text for line in ordered), log_probability, tokens
            ).confidence,
        )
