#!/usr/bin/env python3
"""Render confidence-filtered README examples using actual CLI recognition results."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import subprocess
from pathlib import Path

import cv2
import numpy as np
from PIL import Image, ImageChops, ImageDraw, ImageFont, ImageOps

ROOT = Path(__file__).resolve().parents[1]
CASES = [
    ("mixed", "testdata/720p/mixed-720p-a.png"),
    ("book", "testdata/paddleocr/720p-book.png"),
    ("formula", "testdata/paddleocr/720p-doc_with_formula.png"),
    ("table", "testdata/paddleocr/720p-medal_table.png"),
]
SCALE = 2


def text_patch(line: dict, width: int, height: int, font_path: Path) -> Image.Image:
    rotation = line.get("rotation_degrees")
    if rotation is None:
        rotation = 270 if line["vertical"] and height > width else 0
        if line["rotated_180"]:
            rotation = (rotation + 180) % 360
    if rotation not in (0, 90, 180, 270):
        raise ValueError("unsupported crop correction")
    w, h = (height, width) if rotation % 180 else (width, height)
    font = ImageFont.truetype(str(font_path), max(6, round(h * 0.85)))
    bounds = font.getbbox(line["text"])
    patch = Image.new(
        "RGBA", (max(1, bounds[2] - bounds[0]), max(1, bounds[3] - bounds[1]))
    )
    ImageDraw.Draw(patch).text(
        (-bounds[0], -bounds[1]), line["text"], font=font, fill=(18, 91, 166, 255)
    )
    # Fit every returned character inside its original region; do not truncate text.
    factor = min(max(1, w - 2) / patch.width, max(1, h * 0.80) / patch.height)
    patch = patch.resize(
        (max(1, round(patch.width * factor)), max(1, round(patch.height * factor))),
        Image.Resampling.LANCZOS,
    )
    tile = Image.new("RGBA", (w, h))
    tile.alpha_composite(
        patch, (max(0, (w - patch.width) // 2), max(0, (h - patch.height) // 2))
    )
    inverse = {
        90: Image.Transpose.ROTATE_90,
        180: Image.Transpose.ROTATE_180,
        270: Image.Transpose.ROTATE_270,
    }
    if rotation:
        tile = tile.transpose(inverse[rotation])
    return tile


def place_text(canvas: Image.Image, line: dict, font_path: Path):
    q = np.asarray(line["quad"], dtype=np.float32) * SCALE
    width = max(2, round(float(np.linalg.norm(q[1] - q[0]))))
    height = max(2, round(float(np.linalg.norm(q[3] - q[0]))))
    tile = text_patch(line, width, height, font_path)
    x, y = np.floor(q.min(axis=0)).astype(int)
    right, bottom = np.ceil(q.max(axis=0)).astype(int)
    source = np.array([[0, 0], [width, 0], [width, height], [0, height]], np.float32)
    matrix = cv2.getPerspectiveTransform(source, (q - [x, y]).astype(np.float32))
    warped = cv2.warpPerspective(
        np.asarray(tile),
        matrix,
        (max(1, right - x + 1), max(1, bottom - y + 1)),
        flags=cv2.INTER_CUBIC,
    )
    image = Image.fromarray(warped)
    canvas.paste(image, (int(x), int(y)), image)


def render(args):
    if not 0 <= args.min_confidence <= 1 or not 0 <= args.min_detection_score <= 1:
        raise ValueError("confidence thresholds must be in [0, 1]")
    args.output.mkdir(parents=True, exist_ok=True)
    cases = CASES
    if args.suite == "paddleocr":
        source_manifest = json.loads((ROOT / "testdata/paddleocr/manifest.json").read_text())
        cases = []
        for item in source_manifest["files"]:
            source = "testdata/paddleocr/" + item["file"]
            if hashlib.sha256((ROOT / source).read_bytes()).hexdigest() != item["sha256"]:
                raise ValueError(f"original source hash mismatch: {source}")
            cases.append((Path(item["file"]).stem, source))
    entries = []
    for name, source in cases:
        command = [args.oneocr, "recognize", "--format", "json"]
        if args.home:
            command += ["--home", str(args.home)]
        raw = subprocess.check_output(command + [str(ROOT / source)], text=True)
        result = json.loads(raw)
        lines = [
            line
            for line in result["lines"]
            if line["confidence"] is not None
            and line["confidence"] >= args.min_confidence
            and line["detection_score"] >= args.min_detection_score
        ]
        original = ImageOps.exif_transpose(Image.open(ROOT / source)).convert("RGB")
        if original.size != (result["width"], result["height"]):
            raise ValueError("result coordinates and input image dimensions differ")
        size = (original.width * SCALE, original.height * SCALE)
        boxes = original.resize(size, Image.Resampling.LANCZOS)
        reconstructed = boxes.copy()
        backing = Image.new("RGBA", size)
        for line in lines:
            quad = [(round(x * SCALE), round(y * SCALE)) for x, y in line["quad"]]
            ImageDraw.Draw(boxes).line(
                quad + [quad[0]],
                fill=(0, 157, 125),
                width=2 * SCALE if len(lines) < 20 else SCALE,
                joint="curve",
            )
            ImageDraw.Draw(backing).polygon(quad, fill=(255, 255, 255, 242))
        # Draw all backings before any text so overlapping regions cannot erase labels.
        reconstructed = Image.alpha_composite(
            reconstructed.convert("RGBA"), backing
        ).convert("RGB")
        for line in lines:
            place_text(reconstructed, line, args.font)
        difference = ImageChops.difference(
            original, Image.new("RGB", original.size, "white")
        ).convert("L")
        content = difference.point(lambda x: 255 if x > 18 else 0).getbbox() or (
            0,
            0,
            *original.size,
        )
        xs = [p[0] for line in lines for p in line["quad"]]
        ys = [p[1] for line in lines for p in line["quad"]]
        crop = [
            max(0, math.floor(min([content[0], *xs])) - 18),
            max(0, math.floor(min([content[1], *ys])) - 18),
            min(original.width, math.ceil(max([content[2], *xs])) + 18),
            min(original.height, math.ceil(max([content[3], *ys])) + 18),
        ]
        if args.suite == "paddleocr":
            crop = [0, 0, original.width, original.height]
        scaled_crop = tuple(value * SCALE for value in crop)
        boxes = boxes.crop(scaled_crop)
        reconstructed = reconstructed.crop(scaled_crop)
        assert boxes.size == reconstructed.size
        for suffix, image in [("boxes", boxes), ("text", reconstructed)]:
            image.save(args.output / f"{name}-{suffix}.webp", lossless=True, method=6)
        # Keep exact results and the presentation transform reviewable and reproducible.
        (args.output / f"{name}.json").write_text(
            json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
        )
        entries.append(
            {
                "name": name,
                "source": source,
                "source_size": list(original.size),
                "source_sha256": hashlib.sha256(
                    (ROOT / source).read_bytes()
                ).hexdigest(),
                "recognized_lines": len(result["lines"]),
                "displayed_lines": len(lines),
                "crop": crop,
                "display_scale": SCALE,
                "image_size": list(boxes.size),
                "boxes": f"{name}-boxes.webp",
                "text": f"{name}-text.webp",
                "result": f"{name}.json",
            }
        )
        print(
            f"{name}: {len(lines)}/{len(result['lines'])} lines; paired images {boxes.size}",
            flush=True,
        )
    metadata = {
        "suite": args.suite,
        "recognition_confidence_min": args.min_confidence,
        "detection_score_min": args.min_detection_score,
        "confidence_method": "ctc_token_geometric_mean",
        "font_family": ImageFont.truetype(str(args.font), 20).getname()[0],
        "examples": entries,
    }
    (args.output / "manifest.json").write_text(
        json.dumps(metadata, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    if args.suite == "paddleocr":
        from paddleocr_results import write_pages

        write_pages(metadata, args.output)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--oneocr", default="oneocr", help="installed OneOCR CLI")
    parser.add_argument("--suite", choices=("readme", "paddleocr"), default="readme",
                        help="main README examples or all original PaddleOCR images")
    parser.add_argument(
        "--home", type=Path, help="already prepared OneOCR installation"
    )
    parser.add_argument(
        "--font",
        type=Path,
        required=True,
        help="local font supporting the sample writing systems",
    )
    parser.add_argument("--output", type=Path)
    parser.add_argument("--min-confidence", type=float, default=0.70)
    parser.add_argument("--min-detection-score", type=float, default=0.70)
    args = parser.parse_args()
    if args.output is None:
        args.output = ROOT / "docs/assets" / ("paddleocr-results" if args.suite == "paddleocr" else "ocr-results")
    render(args)


if __name__ == "__main__":
    main()
