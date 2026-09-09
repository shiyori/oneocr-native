#!/usr/bin/env python3
"""Build the core wheel/sdist and a complete offline Python bundle."""
from __future__ import annotations
import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from build_sdk import archive, copy, copy_docs, write_record, digest
from runtime_assets import current_platform
from version import ROOT, VERSION, TAG, PYTHON_VERSION, RUNTIME_VERSION

BOOTSTRAP = '''"""Install this complete Python SDK from any working directory, without network."""
from pathlib import Path
import json
import platform
import subprocess
import sys
import hashlib

root = Path(__file__).resolve().parent
version = f"{sys.version_info.major}.{sys.version_info.minor}"
if version not in {"3.11", "3.12", "3.13"}:
    raise SystemExit("This bundle supports Python 3.11, 3.12 and 3.13")
system = {"Darwin": "darwin", "Windows": "windows", "Linux": "linux"}.get(platform.system())
arch = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64", "amd64": "amd64"}.get(platform.machine().lower())
record = json.loads((root / "SDK.json").read_text(encoding="utf-8"))
if record["platform"] != f"{system}-{arch}":
    raise SystemExit(f"This bundle targets {record['platform']}; download the bundle for {system}-{arch}")
for item in record["files"]:
    path = root / item["file"]
    if not path.resolve().is_relative_to(root) or not path.is_file():
        raise SystemExit("Invalid SDK file: " + item["file"])
    with path.open("rb") as stream:
        actual = hashlib.file_digest(stream, "sha256").hexdigest()
    if path.stat().st_size != item["bytes"] or actual != item["sha256"]:
        raise SystemExit("SDK checksum mismatch: " + item["file"])
wheels = list(root.glob("oneocr_native-*-py3-none-any.whl"))
if len(wheels) != 1:
    raise SystemExit("The OneOCR wheel is missing or ambiguous")
subprocess.run([sys.executable, "-m", "pip", "install", "--no-index", "--find-links", str(root / "wheelhouse" / version), str(wheels[0])], check=True)
subprocess.run([sys.executable, "-m", "oneocr_native", "install", "--source", str(root), "--offline"], check=True)
'''


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT / "dist/release")
    parser.add_argument("--wheel-only", action="store_true")
    args = parser.parse_args()
    args.output = args.output.resolve()
    args.output.mkdir(parents=True, exist_ok=True)
    stamp = subprocess.check_output(["git", "show", "-s", "--format=%ct", "HEAD"], cwd=ROOT, text=True).strip()
    env = dict(os.environ, SOURCE_DATE_EPOCH=stamp)
    subprocess.run(["uv", "build", "--project", str(ROOT / "python"), "--out-dir", str(args.output)], check=True, env=env)
    wheel = args.output / f"oneocr_native-{PYTHON_VERSION}-py3-none-any.whl"
    if args.wheel_only:
        return
    target = current_platform()
    with tempfile.TemporaryDirectory(prefix=".python-sdk-", dir=args.output) as temporary:
        stage = Path(temporary) / f"oneocr-python-{target}"
        stage.mkdir()
        copy(wheel, stage / wheel.name)
        copy_docs(stage)
        for readme in ("README.md", "README.en.md", "README.ja.md"):
            (stage / readme).write_text((ROOT / "python" / readme).read_text(encoding="utf-8").replace(f"](https://github.com/shiyori/oneocr-native/blob/{TAG}/docs/", "](docs/"), encoding="utf-8")
        copy(ROOT / "models/oneocr-cjk-en.ocrpack", stage / "models/oneocr-cjk-en.ocrpack")
        copy(ROOT / "models/LICENSE", stage / "models/LICENSE")
        (stage / "installation.py").write_text(BOOTSTRAP, encoding="utf-8")
        for version in ("3.11", "3.12", "3.13"):
            directory = stage / "wheelhouse" / version
            directory.mkdir(parents=True)
            subprocess.run([sys.executable, "-m", "pip", "download", "--disable-pip-version-check", "--only-binary=:all:",
                            "--python-version", version, "--implementation", "cp", "--abi", "cp" + version.replace(".", ""),
                            "--dest", str(directory), str(wheel), f"onnxruntime=={RUNTIME_VERSION}"], check=True)
        write_record(stage, target, python_versions=["3.11", "3.12", "3.13"], runtime_included=True, models_included=True)
        output = args.output / f"{stage.name}-{VERSION}.zip"
        archive(stage, output)
        print(json.dumps({"file": str(output), "bytes": output.stat().st_size, "sha256": digest(output)}))
if __name__ == "__main__":
    main()
