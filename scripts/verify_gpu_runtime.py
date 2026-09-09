#!/usr/bin/env python3
"""Verify CPU OCR and host session survival using an existing GPU ORT distribution."""
from __future__ import annotations
import argparse
import json
import os
import subprocess
import tempfile
from pathlib import Path
from runtime_assets import current_platform
from version import ROOT

PROBE = r'''
import importlib.metadata, json, sys
from pathlib import Path
import numpy as np
import onnxruntime as ort
from oneocr_native import OneOcrEngine
from oneocr_native.install import install
root, output = map(Path, sys.argv[1:])
assert importlib.metadata.version("onnxruntime-gpu") == "1.26.0"
try:
    importlib.metadata.version("onnxruntime")
    raise AssertionError("CPU distribution was added")
except importlib.metadata.PackageNotFoundError:
    pass
host = ort.InferenceSession(str(root / "testdata/identity.onnx"), providers=["CPUExecutionProvider"])
def check_host():
    assert host.run(None, {"x":np.array([42], dtype=np.float32)})[0][0] == 42
check_host()
install(root, offline=True)
with OneOcrEngine() as engine:
    assert engine.detector.model.get_providers() == ["CPUExecutionProvider"]
    assert engine.recognize(root / "testdata/CJK.png").text == "你好世界 日本語テスト 한국어 123"
    assert engine.detect(root / "testdata/CJK.png").regions
check_host()
assert importlib.metadata.version("onnxruntime-gpu") == "1.26.0"
try:
    importlib.metadata.version("onnxruntime")
    raise AssertionError("Installer replaced GPU ORT with CPU ORT")
except importlib.metadata.PackageNotFoundError:
    pass
capi = Path(ort.__file__).parent / "capi"
libraries = [p for p in capi.iterdir() if p.name == "onnxruntime.dll" or p.name.startswith("libonnxruntime.so")]
assert len(libraries) == 1, libraries
output.write_text(json.dumps({"library":str(libraries[0]), "runtime":ort.__version__, "host_survived":True, "gpu_distribution_kept":True, "cpu_provider":True}), encoding="utf-8")
'''


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    require_platform = current_platform()
    if require_platform not in {"windows-amd64", "linux-amd64"}:
        raise RuntimeError("GPU ORT wheel verification requires Windows/Linux x64")
    if not Path(os.environ.get("ONEOCR_BUNDLE", "missing")).is_dir() or not os.environ.get("ONEOCR_FIXTURES"):
        raise RuntimeError("set ONEOCR_BUNDLE and ONEOCR_FIXTURES so native host tests cannot skip")
    with tempfile.TemporaryDirectory(prefix="oneocr-gpu-host-") as temporary:
        temporary = Path(temporary)
        venv = temporary / "venv"
        subprocess.run(["uv", "venv", "--python", "3.13", str(venv)], check=True)
        python = venv / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
        subprocess.run(["uv", "pip", "install", "--python", str(python), str(ROOT / "python"), "onnxruntime-gpu==1.26.0"], check=True)
        probe = temporary / "probe.py"
        probe.write_text(PROBE, encoding="utf-8")
        env = dict(os.environ, ONEOCR_HOME=str(temporary / "home"), PYTHONUTF8="1")
        subprocess.run([str(python), str(probe), str(ROOT), str(output / "gpu.json")], cwd=temporary, env=env, check=True)
        report = json.loads((output / "gpu.json").read_text(encoding="utf-8"))
        env["ONEOCR_RUNTIME"] = report["library"]
        with (output / "native-gpu.log").open("w", encoding="utf-8") as log:
            subprocess.run(["go", "test", "-run", "^TestNativeLifecycle$", "-count=1", "./internal/engine"], cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
        report.pop("library")
        report.update(native_host_survived=True, platform=require_platform, passed=True)
        (output / "gpu.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")

if __name__ == "__main__":
    main()
