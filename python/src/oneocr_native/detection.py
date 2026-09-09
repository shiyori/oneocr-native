from __future__ import annotations

from dataclasses import dataclass
from typing import TYPE_CHECKING

import cv2
import numpy as np

if TYPE_CHECKING:
    import onnxruntime as ort

from .config import PipelineConfig
from .errors import UnsupportedModelError
from .geometry import ordered_quad, polygon_iou


@dataclass
class Detection:
    quad: np.ndarray
    score: float
    vertical: bool


NEIGHBORS = ((-1, -1), (-1, 0), (-1, 1), (0, -1), (0, 1), (1, -1), (1, 0), (1, 1))


def linked_components(mask: np.ndarray, links: np.ndarray, threshold: float = 0.8):
    """Bidirectional-OR eight-neighbor traversal (DLL RVA 0x5afa30)."""
    if links.shape != (8, *mask.shape):
        raise UnsupportedModelError("unexpected RelationRCNN link head shape")
    remaining = mask.astype(bool).copy()
    height, width = remaining.shape
    for y0, x0 in zip(*np.where(remaining)):
        if not remaining[y0, x0]:
            continue
        remaining[y0, x0] = False
        stack = [(int(y0), int(x0))]
        points = []
        while stack:
            y, x = stack.pop()
            points.append((y, x))
            for channel, (dy, dx) in enumerate(NEIGHBORS):
                ny, nx = y + dy, x + dx
                if not (0 <= ny < height and 0 <= nx < width and remaining[ny, nx]):
                    continue
                if links[channel, y, x] >= threshold or links[7 - channel, ny, nx] >= threshold:
                    remaining[ny, nx] = False
                    stack.append((ny, nx))
        coordinates = np.asarray(points, dtype=np.int32)
        yield coordinates[:, 0], coordinates[:, 1]


def decode_segments(
    score_map: np.ndarray,
    deltas: np.ndarray,
    stride: int,
    threshold: float,
    line_threshold: float,
    links: np.ndarray,
) -> list[tuple[np.ndarray, float]]:
    """Merge positive FPN segments and fit their predicted quadrilateral extent.

    Recovered from DLL RVAs 0x5a8dc0 and 0x5b0500: center offset is
    (stride - 1)/2; regression scale is 8*stride - 1. Eight-neighbor links
    determine connectivity. The final rectangle fit/NMS are independent of
    the DLL's multi-stage proposal fitter and remain experimental.
    """
    if deltas.shape != (8, *score_map.shape):
        raise UnsupportedModelError("unexpected RelationRCNN quadrilateral head shape")
    mask = (score_map >= threshold).astype(np.uint8)
    result = []
    for ys, xs in linked_components(mask, links):
        score = float(score_map[ys, xs].mean())
        if score < line_threshold:
            continue
        offsets = deltas[:, ys, xs].T.reshape(-1, 4, 2)
        centers = np.stack((xs, ys), axis=1)[:, None, :] * stride + (stride - 1) / 2
        corners = centers + offsets * (8 * stride - 1)
        if not np.isfinite(corners).all():
            continue
        quad = ordered_quad(
            cv2.boxPoints(cv2.minAreaRect(corners.astype(np.float32).reshape(-1, 2)))
        )
        if cv2.contourArea(quad) < 12:
            continue
        result.append((quad, score))
    return result


class Detector:
    def __init__(self, model: ort.InferenceSession, config: PipelineConfig, max_side: int):
        self.model = model
        self.config = config
        self.max_side = max_side
        names = {item.name for item in model.get_outputs()}
        needed = {
            f"{head}_{direction}_fpn{level}"
            for head in ("scores", "bbox_deltas", "link_scores")
            for direction in ("hori", "vert")
            for level in (2, 3, 4)
        }
        if not needed <= names:
            raise UnsupportedModelError(
                "model does not expose the expected P2/P3/P4 detector heads"
            )

    def run(self, rgb: np.ndarray) -> list[Detection]:
        h, w = rgb.shape[:2]
        # Downscale large documents; never distort the aspect ratio or squash
        # small screenshots into a fixed square.
        scale = min(1.0, self.max_side / max(h, w))
        resized_w, resized_h = max(1, round(w * scale)), max(1, round(h * scale))
        resized = cv2.resize(rgb, (resized_w, resized_h), interpolation=cv2.INTER_AREA)
        pad_w, pad_h = (32 - resized_w % 32) % 32, (32 - resized_h % 32) % 32
        padded = cv2.copyMakeBorder(
            resized, 0, pad_h, 0, pad_w, cv2.BORDER_CONSTANT, value=(255, 255, 255)
        )
        # ImageNet RGB mean/std are already embedded in the detector graph.
        values = self.model.run(
            None,
            {
                "data": padded.astype(np.float32).transpose(2, 0, 1)[None].copy(),
                "im_info": np.array([[resized_h, resized_w, 1]], dtype=np.float32),
            },
        )
        outputs = {item.name: value for item, value in zip(self.model.get_outputs(), values)}
        candidates = []
        for level in (2, 3, 4):
            for direction in ("hori", "vert"):
                groups = decode_segments(
                    outputs[f"scores_{direction}_fpn{level}"][0, 0],
                    outputs[f"bbox_deltas_{direction}_fpn{level}"][0],
                    2**level,
                    self.config.segment_threshold,
                    self.config.line_thresholds[level],
                    outputs[f"link_scores_{direction}_fpn{level}"][0],
                )
                for quad, confidence in groups:
                    quad[:, 0] *= w / resized_w
                    quad[:, 1] *= h / resized_h
                    quad[:, 0] = np.clip(quad[:, 0], 0, w - 1)
                    quad[:, 1] = np.clip(quad[:, 1], 0, h - 1)
                    if cv2.contourArea(quad) >= 12:
                        candidates.append(Detection(quad, confidence, direction == "vert"))
        candidates.sort(key=lambda item: item.score, reverse=True)
        kept: list[Detection] = []
        for candidate in candidates:
            if not any(polygon_iou(candidate.quad, prior.quad) > 0.2 for prior in kept):
                kept.append(candidate)
        return kept
