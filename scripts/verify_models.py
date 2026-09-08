#!/usr/bin/env python3
"""Verify tracked model payloads without importing an OCR SDK."""

import hashlib
from pathlib import Path

root = Path(__file__).resolve().parents[1] / "models"
entries = (root / "SHA256SUMS").read_text().splitlines()
listed = {line.split()[1] for line in entries}
actual_files = {path.name for path in root.glob("*.ocrpack")}
if actual_files != listed:
    raise SystemExit(
        f"model manifest mismatch: unlisted={sorted(actual_files - listed)}, "
        f"missing={sorted(listed - actual_files)}"
    )
for line in entries:
    expected, name = line.split()
    with (root / name).open("rb") as stream:
        actual = hashlib.file_digest(stream, "sha256").hexdigest()
    if actual != expected:
        raise SystemExit(f"checksum mismatch: {name}")
    print(f"OK {name}")
