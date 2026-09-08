import cv2
import numpy as np

from oneocr_native.detection import decode_segments, linked_components
from oneocr_native.geometry import ordered_quad, polygon_iou, reading_order, rectify


def box(x, y, width, height):
    return np.array([[x, y], [x + width, y], [x + width, y + height], [x, y + height]], np.float32)


def test_column_order_and_rtl():
    quads = [box(0, 0, 100, 20), box(200, 0, 100, 20), box(0, 80, 100, 20), box(200, 80, 100, 20)]
    assert reading_order(quads) == [0, 2, 1, 3]
    assert reading_order(quads, rtl=True) == [1, 3, 0, 2]


def test_polygon_iou_does_not_use_axis_aligned_overlap():
    a = box(0, 0, 10, 10)
    assert polygon_iou(a, a) == 1
    assert polygon_iou(a, box(5, 0, 10, 10)) == 1 / 3
    assert polygon_iou(a, box(20, 0, 10, 10)) == 0


def test_corner_order_and_rectification():
    image = np.zeros((40, 60, 3), dtype=np.uint8)
    image[:20, :30] = (255, 0, 0)
    shuffled = np.array([[59, 39], [0, 0], [0, 39], [59, 0]], np.float32)
    quad = ordered_quad(shuffled)
    np.testing.assert_array_equal(quad, box(0, 0, 59, 39))
    crop = rectify(image, shuffled)
    np.testing.assert_array_equal(crop[5, 5], [255, 0, 0])
    np.testing.assert_array_equal(crop[-5, -5], [0, 0, 0])


def test_rotated_quad_is_convex_and_clockwise():
    quad = cv2.boxPoints(((50, 50), (60, 20), 35))
    ordered = ordered_quad(quad)
    assert cv2.isContourConvex(ordered)
    assert cv2.contourArea(ordered, oriented=True) > 0


def test_decodes_known_segment_anchor_offsets():
    scores = np.zeros((8, 8), np.float32)
    scores[3, 3:5] = 0.9
    deltas = np.zeros((8, 8, 8), np.float32)
    delta = np.array([-0.5, -0.5, 0.5, -0.5, 0.5, 0.5, -0.5, 0.5], np.float32)
    deltas[:, 3, 3] = delta
    deltas[:, 3, 4] = delta
    output = decode_segments(scores, deltas, 4, 0.7, 0.8, np.ones_like(deltas))
    assert len(output) == 1
    np.testing.assert_allclose(output[0][0], box(-2, -2, 35, 31), atol=1e-5)


def test_empty_detector_map():
    assert (
        decode_segments(
            np.zeros((8, 8), np.float32),
            np.zeros((8, 8, 8), np.float32),
            4,
            0.7,
            0.8,
            np.zeros((8, 8, 8), np.float32),
        )
        == []
    )


def test_disconnected_neighbors_require_an_explicit_link_in_either_direction():
    mask = np.array([[1, 1]], np.uint8)
    links = np.zeros((8, 1, 2), np.float32)
    assert len(list(linked_components(mask, links))) == 2
    # Channel 3 is left: only the second cell points back to the first.
    links[3, 0, 1] = 0.9
    groups = list(linked_components(mask, links))
    assert len(groups) == 1
    assert set(zip(*groups[0])) == {(0, 0), (0, 1)}
