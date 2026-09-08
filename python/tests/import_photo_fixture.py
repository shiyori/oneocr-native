"""Import a small annotated crop from a locally supplied public OneOCR sample.

No network access. Obtain ocr-book.jpg from the cited source separately. The
sample image is not redistributed with this package. Fullwidth colon is the
annotation convention; punctuation differences count as errors.
"""

import json
import sys
from pathlib import Path

from PIL import Image

SOURCE = "https://github.com/b1tg/win11-oneocr/blob/master/ocr-book.jpg"
CROP = (72, 378, 300, 436)


def import_photo(source: Path, root: Path):
    with Image.open(source) as image:
        if image.size != (1123, 781):
            raise ValueError(
                "expected the 1123×781 public sample; crop coordinates would be invalid"
            )
        image.crop(CROP).save(root / "book_photo_crop.png")
    path = root / "annotations.json"
    annotations = [
        case for case in json.loads(path.read_text()) if case["file"] != "book_photo_crop.png"
    ]
    annotations.append(
        {
            "file": "book_photo_crop.png",
            "text": "试着运行一下脚本：\n$ python basic_avast_cli.py\n'220 DAEMON\\r\\n'",
            "source": SOURCE,
            "crop": list(CROP),
        }
    )
    path.write_text(json.dumps(annotations, ensure_ascii=False, indent=2) + "\n")


if __name__ == "__main__":
    import_photo(Path(sys.argv[1]), Path(sys.argv[2]))
