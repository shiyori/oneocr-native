"""Add deterministic layout/rotation/contrast cases to CoreText-rendered fixtures."""

import json
import sys
from pathlib import Path

from PIL import Image, ImageEnhance


def generate(root: Path):
    annotations = [
        item
        for item in json.loads((root / "annotations.json").read_text())
        if not item.get("derived")
    ]

    def add(name, image, text):
        image.save(root / name)
        annotations.append({"file": name, "text": text, "derived": True})

    latin = Image.open(root / "Latin.png").convert("RGB")
    for angle in (90, 180, 270, 12):
        add(
            f"Latin_rotate_{angle}.png",
            latin.rotate(angle, expand=True, fillcolor="white"),
            "Hello World 123",
        )
    add("low_contrast.png", ImageEnhance.Contrast(latin).enhance(0.15), "Hello World 123")
    add("blank.png", Image.new("RGB", (800, 400), "white"), "")
    left = latin.crop((0, 0, 440, 160))
    right = Image.open(root / "Cyrillic.png").convert("RGB").crop((0, 0, 440, 160))
    columns = Image.new("RGB", (1020, 520), "white")
    for y in (10, 180, 350):
        columns.paste(left, (10, y))
        columns.paste(right, (570, y))
    column_text = "\n".join(["Hello World 123"] * 3 + ["Привет мир 123"] * 3)
    add("columns.png", columns, column_text)
    add("columns_rotate_90.png", columns.rotate(90, expand=True, fillcolor="white"), column_text)
    multilingual = Image.new("RGB", (1000, 9 * 160), "white")
    for index, case in enumerate(annotations[:9]):
        with Image.open(root / case["file"]) as line:
            multilingual.paste(line, (0, 160 * index))
    add("multilingual.png", multilingual, "\n".join(case["text"] for case in annotations[:9]))
    (root / "annotations.json").write_text(
        json.dumps(annotations, ensure_ascii=False, indent=2) + "\n"
    )


if __name__ == "__main__":
    generate(Path(sys.argv[1]))
