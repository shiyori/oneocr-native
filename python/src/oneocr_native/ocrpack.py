"""Verified ONEOCRPK v1 resource reader; no extraction or Go library required."""

from __future__ import annotations

import hashlib
import json
import math
import os
import posixpath
import re
import stat
import struct
from pathlib import Path
from threading import RLock

from .config import CharacterModel, PipelineConfig
from .errors import ModelFormatError, OneOcrError

MAGIC = b"ONEOCRPK"
MAX_BYTES = 2 * 1024**3
PROFILES = {"cjk-en": {"CJK", "Latin"}, "extended": {"CJK", "Latin", "Cyrillic", "Arabic"}}


def _require(condition, message):
    if not condition:
        raise ModelFormatError(f"ocrpack: {message}")


def _file_metadata(entry):
    name = entry["file"]
    _require(
        isinstance(name, str)
        and name not in ("", ".", "..")
        and not any(c in name for c in "\\:\x00")
        and not name.startswith(("/", "../"))
        and posixpath.normpath(name) == name,
        "invalid resource path",
    )
    _require(
        isinstance(entry["sha256"], str) and re.fullmatch(r"[a-fA-F0-9]{64}", entry["sha256"]),
        "invalid SHA-256",
    )
    _require(type(entry["bytes"]) is int and 0 <= entry["bytes"] <= MAX_BYTES, "invalid size")


class PackageSource:
    """Owns one read-only descriptor and verifies resources before ORT sees them."""

    def __init__(self, filename: str | Path):
        self._lock = RLock()
        self._file = open(filename, "rb")  # noqa: SIM115 — owned until close()
        try:
            self._load()
        except Exception as exc:
            self.close()
            if isinstance(
                exc, (KeyError, TypeError, ValueError, OSError, AttributeError, struct.error)
            ):
                raise ModelFormatError(f"invalid ocrpack: {exc}") from exc
            raise

    def _load(self):
        status = os.fstat(self._file.fileno())
        _require(
            stat.S_ISREG(status.st_mode) and 64 <= status.st_size <= MAX_BYTES,
            "invalid file type or size",
        )
        magic, version, flags, index_size, offset, digest = struct.unpack(
            "<8sIIQQ32s", self._file.read(64)
        )
        _require(magic == MAGIC and version == 1 and flags == 0, "unsupported header")
        _require(
            0 < index_size <= 16 * 1024**2
            and offset == ((64 + index_size + 63) & ~63)
            and offset <= status.st_size,
            "invalid index bounds",
        )
        raw = self._file.read(index_size)
        _require(hashlib.sha256(raw).digest() == digest, "index checksum mismatch")
        self.info = json.loads(raw)
        _require(self.info["schema"] == "oneocr.pack.v1", "unsupported schema")
        allowed = PROFILES.get(self.info["profile"])
        _require(allowed is not None, "unsupported profile")
        self.manifest = b = self.info["bundle"]
        _require(
            b["schema"] == "oneocr.bundle.v1"
            and isinstance(b["source_sha256"], str)
            and re.fullmatch(r"[a-fA-F0-9]{64}", b["source_sha256"]),
            "invalid bundle",
        )
        p = b["pipeline"]
        chars = tuple(CharacterModel(**c) for c in p["characters"])
        _require(
            len(chars) == len(allowed) and {c.script for c in chars} == allowed,
            "profile and recognizers disagree",
        )
        _require(
            all(c.model_path and c.alphabet_path and c.pixels_per_frame in (4, 8) for c in chars),
            "invalid recognizer",
        )
        levels = {int(k): v for k, v in p["line_thresholds"].items()}
        _require({2, 3, 4} <= levels.keys(), "missing line thresholds")
        _require(
            all(
                isinstance(v, (float, int))
                and not isinstance(v, bool)
                and math.isfinite(v)
                and 0 < v <= 1
                for v in [p["segment_threshold"], *(levels[k] for k in (2, 3, 4))]
            ),
            "invalid threshold",
        )
        self.config = PipelineConfig(
            p["detector_path"], p["classifier_path"], chars, p["segment_threshold"], levels
        )
        _file_metadata(b["config"])
        expected = {b["config"]["file"]: b["config"]}
        ids, resources = set(), set()
        _require(0 < len(b["resources"]) <= 4096, "invalid resource count")
        for r in b["resources"]:
            _file_metadata(r)
            _require(
                r["file"] not in expected
                and type(r["id"]) is int
                and r["id"] >= 0
                and r["id"] not in ids
                and r["kind"] in ("onnx", "data"),
                "duplicate or invalid resource",
            )
            ids.add(r["id"])
            resources.add(r["file"])
            expected[r["file"]] = r
        _require(resources == self.config.required_paths(), "invalid dependency closure")
        _require("pipeline.spec.json" not in expected, "reserved specification name")
        _require(len(self.info["files"]) == len(resources) + 2, "incorrect file count")
        self.entries = {}
        self._offset = offset
        previous, end, seen = "", 0, set()
        for entry in self.info["files"]:
            _file_metadata(entry)
            name = entry["file"]
            _require(name > previous and name.lower() not in seen, "unsorted or duplicate name")
            _require(
                type(entry["offset"]) is int
                and entry["offset"] == ((end + 63) & ~63)
                and entry["offset"] + entry["bytes"] <= status.st_size - offset,
                "invalid resource extent",
            )
            wanted = expected.pop(name, None)
            if wanted is None:
                _require(name == "pipeline.spec.json", "unlisted file")
            else:
                _require(
                    wanted["bytes"] == entry["bytes"]
                    and wanted["sha256"].lower() == entry["sha256"].lower(),
                    "inconsistent resource metadata",
                )
            self.entries[name] = entry
            previous = name
            seen.add(name.lower())
            end = entry["offset"] + entry["bytes"]
        _require(
            not expected
            and "pipeline.spec.json" in self.entries
            and end == status.st_size - offset,
            "uncovered file/data section",
        )
        for entry in self.entries.values():
            self._file.seek(offset + entry["offset"])
            digest = hashlib.sha256()
            remaining = entry["bytes"]
            while remaining:
                data = self._file.read(min(65536, remaining))
                _require(bool(data), "truncated resource")
                digest.update(data)
                remaining -= len(data)
            _require(digest.hexdigest() == entry["sha256"].lower(), "resource checksum mismatch")

    def read(self, name: str) -> bytes:
        with self._lock:
            if self._file.closed:
                raise OneOcrError("model package is closed")
            entry = self.entries.get(name)
            _require(entry is not None, f"missing resource: {name}")
            self._file.seek(self._offset + entry["offset"])
            data = self._file.read(entry["bytes"])
            _require(
                len(data) == entry["bytes"]
                and hashlib.sha256(data).hexdigest() == entry["sha256"].lower(),
                "resource changed or checksum mismatch",
            )
            return data

    def close(self):
        with self._lock:
            self._file.close()

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
