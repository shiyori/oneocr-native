from __future__ import annotations

import cv2
import numpy as np


def ordered_quad(points: np.ndarray) -> np.ndarray:
    """Clockwise TL, TR, BR, BL for a convex text rectangle."""
    points = np.asarray(points, dtype=np.float32).reshape(4, 2)
    center = points.mean(axis=0)
    angles = np.arctan2(points[:, 1] - center[1], points[:, 0] - center[0])
    points = points[np.argsort(angles)]
    start = int(np.argmin(points.sum(axis=1)))
    return np.roll(points, -start, axis=0).copy()


def polygon_iou(a: np.ndarray, b: np.ndarray) -> float:
    a = cv2.convexHull(np.asarray(a, np.float32))
    b = cv2.convexHull(np.asarray(b, np.float32))
    area_a, area_b = cv2.contourArea(a), cv2.contourArea(b)
    if min(area_a, area_b) <= 0:
        return 0.0
    intersection, _ = cv2.intersectConvexConvex(a, b)
    return float(intersection / max(area_a + area_b - intersection, 1e-6))


def rectify(image: np.ndarray, quad: np.ndarray, *, vertical: bool = False) -> np.ndarray:
    points = ordered_quad(quad)
    width = max(np.linalg.norm(points[1] - points[0]), np.linalg.norm(points[2] - points[3]))
    height = max(np.linalg.norm(points[3] - points[0]), np.linalg.norm(points[2] - points[1]))
    w, h = max(2, round(float(width))), max(2, round(float(height)))
    target = np.array([[0, 0], [w - 1, 0], [w - 1, h - 1], [0, h - 1]], np.float32)
    transform = cv2.getPerspectiveTransform(points, target)
    crop = cv2.warpPerspective(
        image, transform, (w, h), flags=cv2.INTER_LINEAR, borderMode=cv2.BORDER_REPLICATE
    )
    if vertical and h > w:
        crop = cv2.rotate(crop, cv2.ROTATE_90_COUNTERCLOCKWISE)
    return crop


def reading_order(quads: list[np.ndarray], *, rtl: bool = False) -> list[int]:
    """Recursive whitespace cuts: spanning titles, columns, then text rows.

    This is an independent layout implementation, not the original DLL's
    layout engine. It uses original image geometry rather than OCR language.
    """
    if not quads:
        return []
    bounds = np.array([[q[:, 0].min(), q[:, 1].min(), q[:, 0].max(), q[:, 1].max()] for q in quads])

    def cut(indices: list[int]) -> list[int]:
        if len(indices) <= 1:
            return indices
        height = float(np.median(bounds[indices, 3] - bounds[indices, 1]))
        candidates = []
        for axis in (0, 1):
            ordered = sorted(indices, key=lambda i: bounds[i, axis])
            edge = bounds[ordered[0], axis + 2]
            for split, idx in enumerate(ordered[1:], 1):
                gap = bounds[idx, axis] - edge
                # A column gutter must be substantially wider than a word gap.
                required = max(2, height * (1.5 if axis == 0 else 0.2))
                if gap > required:
                    candidates.append((gap / required, axis, split, ordered))
                edge = max(edge, bounds[idx, axis + 2])
        if candidates:
            # A full-height gutter separates columns; reading across row gaps
            # first would interleave the columns of a normal document.
            columns = [candidate for candidate in candidates if candidate[1] == 0]
            _, axis, split, ordered = max(columns or candidates, key=lambda x: x[0])
            groups = [ordered[:split], ordered[split:]]
            if axis == 0 and rtl:
                groups.reverse()
            return cut(groups[0]) + cut(groups[1])
        return sorted(
            indices, key=lambda i: (float(bounds[i, 1]), float(bounds[i, 0]) * (-1 if rtl else 1))
        )

    return cut(list(range(len(quads))))
