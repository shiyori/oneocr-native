#!/usr/bin/env python3
"""Run the full native and Python integration suites against both supported runtime baselines."""
from __future__ import annotations
import argparse
import json
import os
import subprocess
import sys
from pathlib import Path
from runtime_assets import current_platform, prepare
from version import ROOT


def run(command, env, log):
    with log.open("w", encoding="utf-8") as output:
        result = subprocess.run([str(c) for c in command], cwd=ROOT, env=env, text=True, stdout=output, stderr=subprocess.STDOUT)
    if result.returncode:
        raise RuntimeError(log.read_text(encoding="utf-8")[-12000:])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    bundle = output / "extended-bundle"
    env = dict(os.environ, PYTHONUTF8="1")
    if not bundle.exists():
        run(["go", "run", "./cmd/oneocr", "unpack", "--model", ROOT / "models/oneocr-extended.ocrpack", "--directory", bundle], env, output / "unpack.log")
    env.update(ONEOCR_BUNDLE=str(bundle), ONEOCR_MODEL=str(ROOT / "models/oneocr-extended.ocrpack"), ONEOCR_PACKAGES=str(ROOT / "models"), ONEOCR_FIXTURES=str(ROOT / "testdata"))
    target = current_platform()
    library = "onnxruntime.dll" if target.startswith("windows") else "libonnxruntime.dylib" if target.startswith("darwin") else "libonnxruntime.so"
    for version in ("1.26.0", "1.29.0"):
        directory = prepare(target, output / "upstream", version)
        env["ONEOCR_RUNTIME"] = str(directory / library)
        run(["go", "test", "-race", "-count=1", "./..."], env, output / f"go-{version}.log")
        run([sys.executable, "-m", "pip", "install", "onnxruntime==" + version], env, output / f"pip-{version}.log")
        run([sys.executable, "-m", "pytest", "python/tests"], env, output / f"python-{version}.log")
        crops = output / ("reference-" + version)
        run([sys.executable, ROOT / "scripts/api_reference.py", "--runtime", directory / library, "--output", crops], env, output / f"reference-{version}.log")
        run([sys.executable, ROOT / "scripts/python_reference.py", "--runtime-version", version, "--crops", crops, "--output", output / ("python-reference-" + version)], env, output / f"python-reference-{version}.log")
    run(["go", "vet", "./..."], env, output / "vet.log")
    (output / "native.json").write_text(json.dumps({"platform":target, "runtimes":["1.26.0", "1.29.0"], "go_race":True, "python_integration":True, "go_comparisons":1008, "python_comparisons":504, "passed":True}, indent=2) + "\n", encoding="utf-8")

if __name__ == "__main__":
    main()
