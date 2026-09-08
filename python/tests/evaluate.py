"""Evaluate a labelled image directory without changing or normalizing away errors.

Usage: uv run python tests/evaluate.py --model ... --fixtures ... --output ...
annotations.json is a list of {file, text, script?} records; script is a label,
NOT passed to the engine. Automatic selection is tested.
"""

from __future__ import annotations

import argparse
import json
import platform
import resource
import statistics
import time
import unicodedata
from hashlib import sha256
from pathlib import Path

import onnxruntime as ort

from oneocr_native import OneOcrEngine


def edit_distance(a: str, b: str) -> int:
    previous = list(range(len(b) + 1))
    for row, ca in enumerate(a, 1):
        current = [row]
        for col, cb in enumerate(b, 1):
            current.append(min(current[-1] + 1, previous[col] + 1, previous[col - 1] + (ca != cb)))
        previous = current
    return previous[-1]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True)
    parser.add_argument("--fixtures", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--cache-dir", type=Path)
    args = parser.parse_args()
    begin = time.perf_counter()
    engine = OneOcrEngine(args.model, cache_dir=args.cache_dir)
    report = {
        "platform": platform.platform(),
        "machine": platform.machine(),
        "model_sha256": engine.prepared.manifest["source_sha256"],
        "onnxruntime_version": ort.__version__,
        "provider": "CPUExecutionProvider",
        "threads": 2,
        "cache_reused": engine.prepared.cache_hit,
        "initialization_seconds": time.perf_counter() - begin,
        "cases": [],
    }
    for case in json.loads((args.fixtures / "annotations.json").read_text()):
        result = engine.recognize(args.fixtures / case["file"])
        expected = unicodedata.normalize("NFC", case["text"])
        actual = unicodedata.normalize("NFC", result.text)
        errors = edit_distance(expected, actual)
        record = {
            "file": case["file"],
            "image_sha256": sha256((args.fixtures / case["file"]).read_bytes()).hexdigest(),
            "corpus": "photograph_crop" if case.get("source") else "generated",
            "expected": expected,
            "actual": actual,
            "character_errors": errors,
            "reference_characters": len(expected),
            "exact_match": actual == expected,
            "result": result.to_dict(),
        }
        for field in ("source", "crop"):
            if field in case:
                record[field] = case[field]
        report["cases"].append(record)
        print(case["file"], repr(actual), "errors=", errors, flush=True)
    peak = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
    report["peak_rss_bytes"] = peak if platform.system() == "Darwin" else peak * 1024
    report["exact_matches"] = sum(c["exact_match"] for c in report["cases"])
    report["character_error_rate"] = sum(c["character_errors"] for c in report["cases"]) / max(
        1, sum(c["reference_characters"] for c in report["cases"])
    )
    report["by_corpus"] = {}
    for corpus in sorted({case["corpus"] for case in report["cases"]}):
        cases = [case for case in report["cases"] if case["corpus"] == corpus]
        report["by_corpus"][corpus] = {
            "cases": len(cases),
            "exact_matches": sum(case["exact_match"] for case in cases),
            "character_errors": sum(case["character_errors"] for case in cases),
            "reference_characters": sum(case["reference_characters"] for case in cases),
            "median_seconds": statistics.median(
                case["result"]["elapsed_seconds"] for case in cases
            ),
        }
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")


if __name__ == "__main__":
    main()
