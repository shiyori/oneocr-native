#!/usr/bin/env python3
"""Record reproducible single-image and bounded-batch OCR timings."""
from __future__ import annotations

import argparse
import json
import platform
import statistics
import time
from pathlib import Path

try:
    import resource
except ImportError:  # pragma: no cover - Windows has no resource module
    resource = None


def percentile(values: list[float], p: float) -> float:
    if not values:
        return 0.0
    values = sorted(values)
    index = min(len(values) - 1, round((len(values) - 1) * p))
    return values[index]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("images", nargs="+", type=Path)
    parser.add_argument("--model", type=Path)
    parser.add_argument("--cache-dir", type=Path)
    parser.add_argument("--threads", type=int, default=2)
    parser.add_argument("--max-inflight", type=int, default=1)
    parser.add_argument("--repeats", type=int, default=3)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.repeats < 1:
        parser.error("--repeats must be positive")

    from oneocr_native import EngineConfig, OneOcrEngine

    with OneOcrEngine(
        EngineConfig(
            args.model,
            cache_dir=args.cache_dir,
            threads=args.threads,
            max_inflight=args.max_inflight,
        )
    ) as engine:
        # Load every recognizer before timing so lazy initialization does not
        # dominate the comparison between revisions.
        for image in args.images:
            engine.recognize(image)
        single = []
        batch = []
        for _ in range(args.repeats):
            start = time.perf_counter()
            for image in args.images:
                engine.recognize(image)
            single.append(time.perf_counter() - start)
            start = time.perf_counter()
            engine.recognize_batch(args.images)
            batch.append(time.perf_counter() - start)
        report = {
            "images": [str(path) for path in args.images],
            "repeats": args.repeats,
            "threads": args.threads,
            "max_inflight": args.max_inflight,
            "single_seconds": {"p50": percentile(single, 0.50), "p95": percentile(single, 0.95)},
            "batch_seconds": {"p50": percentile(batch, 0.50), "p95": percentile(batch, 0.95)},
            "batch_images_per_second": len(args.images) / statistics.mean(batch),
            "model_sha256": engine.prepared.manifest["source_sha256"],
            "platform": {"system": platform.system().lower(), "machine": platform.machine()},
            "peak_rss_bytes": (
                resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
                * (1 if platform.system() == "Darwin" else 1024)
                if resource is not None
                else None
            ),
        }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
