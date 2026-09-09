"""Conservative routing for compact numerals and ambiguous crop orientation."""

from __future__ import annotations

import math

import cv2
import numpy as np

from .geometry import ordered_quad
from .recognition import Recognition


def quad_dimensions(quad: np.ndarray) -> tuple[float, float]:
    q = ordered_quad(quad)
    return (
        float(max(np.linalg.norm(q[1] - q[0]), np.linalg.norm(q[2] - q[3]))),
        float(max(np.linalg.norm(q[3] - q[0]), np.linalg.norm(q[2] - q[1]))),
    )


def compact_quad(quad: np.ndarray) -> bool:
    w, h = quad_dimensions(quad)
    return min(w, h) > 0 and max(w, h) <= 2 * min(w, h)


def rotate_quarter(crop: np.ndarray, turns: int) -> np.ndarray:
    rotations = {1: cv2.ROTATE_90_COUNTERCLOCKWISE, 2: cv2.ROTATE_180, 3: cv2.ROTATE_90_CLOCKWISE}
    return cv2.rotate(crop, rotations[turns % 4]) if turns % 4 else crop


def page_orientation(anchors: list[tuple[float, int]]) -> float | None:
    x = sum(min(20, count) * math.cos(angle) for angle, count in anchors)
    y = sum(min(20, count) * math.sin(angle) for angle, count in anchors)
    weight = sum(min(20, count) for _, count in anchors)
    if not weight or math.hypot(x, y) / weight < 0.90:
        return None
    return math.atan2(y, x)


def orientation_turns(quad: np.ndarray, prior: float | None) -> int:
    if prior is None:
        return 0
    q = ordered_quad(quad)
    edge = q[1] - q[0]
    delta = prior - math.atan2(float(edge[1]), float(edge[0]))
    delta = math.atan2(math.sin(delta), math.cos(delta))
    turns = round(delta / (math.pi / 2))
    return turns % 4 if abs(delta - turns * math.pi / 2) <= math.pi / 6 else 0


def ascii_digits(text: str) -> bool:
    return 1 <= len(text) <= 3 and all("0" <= c <= "9" for c in text)


def numeric_consensus(latin: Recognition, cjk: Recognition) -> bool:
    a, b = latin.confidence, cjk.confidence
    return (
        ascii_digits(latin.text)
        and latin.text == cjk.text
        and a is not None
        and b is not None
        and a >= 0.90
        and b >= 0.50
    )
