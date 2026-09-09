#!/usr/bin/env python3
"""Compare Python's three CPU operations with the original API in identical dependencies."""
from __future__ import annotations
import argparse
import json
import os
import subprocess
import tarfile
import tempfile
from pathlib import Path
from api_reference import BASELINE
from version import ROOT

PROBE = r'''
import json, sys
from pathlib import Path
from PIL import Image
from oneocr_native import OneOcrEngine
from oneocr_native.errors import OneOcrError
import onnxruntime
root, crops, mode, output = map(Path, sys.argv[1:])
if str(mode) == "new":
    from oneocr_native import EngineConfig
    engine = OneOcrEngine(EngineConfig(root / "models/oneocr-cjk-en.ocrpack", threads=1))
else:
    engine = OneOcrEngine(root / "models/oneocr-cjk-en.ocrpack", threads=1)
records = []
for index, case in enumerate(json.loads((crops / "cases.json").read_text(encoding="utf-8"))):
    for operation in ("recognize", "detect", "recognize_line"):
        path = crops / case["line_file"] if operation == "recognize_line" else root / case["file"]
        for form in ("file", "PIL"):
            with Image.open(path) as pil:
                try:
                    value = getattr(engine, operation)(path if form == "file" else pil).to_dict()
                    value.pop("elapsed_seconds", None)
                except (OneOcrError, ValueError) as error:
                    value = {"error": type(error).__name__, "message": str(error)}
            records.append({"file":case["file"], "operation":operation, "form":form, "result":value})
    if (index + 1) % 6 == 0:
        print(f"{mode}: {index + 1} fixtures", flush=True)
engine.close()
output.write_text(json.dumps({"runtime":onnxruntime.__version__, "records":records}, ensure_ascii=False, indent=2), encoding="utf-8")
'''


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--crops", type=Path, required=True, help="api_reference.py output")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--runtime-version", required=True, choices=("1.26.0", "1.29.0"))
    args = parser.parse_args()
    args.output = args.output.resolve()
    args.output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="oneocr-python-reference-") as temporary:
        temporary = Path(temporary)
        snapshot = temporary / "source"
        snapshot.mkdir()
        archive = temporary / "source.tar"
        with archive.open("wb") as output:
            subprocess.run(["git", "archive", BASELINE], cwd=ROOT, stdout=output, check=True)
        with tarfile.open(archive) as source:
            source.extractall(snapshot, filter="data")
        environment = temporary / "venv"
        subprocess.run(["uv", "venv", "--python", "3.13", str(environment)], check=True)
        python = environment / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
        subprocess.run(["uv", "pip", "install", "--python", str(python), str(ROOT / "python") + "[conversion]", "onnxruntime==" + args.runtime_version], check=True)
        probe = temporary / "probe.py"
        probe.write_text(PROBE, encoding="utf-8")
        for mode, source in (("old", snapshot), ("new", ROOT)):
            env = dict(os.environ, PYTHONPATH=str(source / "python/src"), ONEOCR_HOME=str(temporary / mode), PYTHONUTF8="1")
            subprocess.run([str(python), str(probe), str(ROOT), str(args.crops.resolve()), mode, str(args.output / (mode + ".json"))], cwd=temporary, env=env, check=True)
        old, new = [json.loads((args.output / (mode + ".json")).read_text(encoding="utf-8")) for mode in ("old", "new")]
        if old != new:
            differences = [a for a, b in zip(old["records"], new["records"], strict=True) if a != b]
            raise RuntimeError(f"Python baseline mismatch: {len(differences)} cases; see old.json/new.json")
        report = {"baseline_commit":BASELINE, "runtime":args.runtime_version, "fixtures":len(new["records"]) // 6, "operations":3, "input_forms":2, "comparisons":len(new["records"]), "passed":True}
        (args.output / "report.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
        print(json.dumps(report))

if __name__ == "__main__":
    main()
