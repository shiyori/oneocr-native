#!/usr/bin/env python3
"""Verify code consumers and complete Linux packages from clean directories."""
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

from runtime_assets import current_platform, digest
from version import PYTHON_VERSION, ROOT, VERSION


def run(*command, cwd: Path, env: dict, success: bool = True):
    result = subprocess.run([str(x) for x in command], cwd=cwd, env=env, text=True, capture_output=True, check=False)
    if success and result.returncode:
        raise RuntimeError(f"{command[0]} failed ({result.returncode}):\n{result.stdout[-4000:]}\n{result.stderr[-8000:]}")
    if not success and result.returncode == 0:
        raise RuntimeError("invalid installation unexpectedly succeeded")
    return result


def isolated(root: Path) -> dict:
    environment = dict(os.environ)
    for name in tuple(environment):
        if name.startswith("ONEOCR_") or name in {"PYTHONPATH", "PYTHONHOME", "UV_PROJECT_ENVIRONMENT", "VIRTUAL_ENV"}:
            environment.pop(name, None)
    environment.update(ONEOCR_HOME=str(root / "oneocr-home"), GOMODCACHE=str(root / "go-modules"),
                       GOPATH=str(root / "go-path"), GOCACHE=str(root / "go-build"), GOWORK="off",
                       GOPROXY="off", GOTOOLCHAIN="local", PIP_NO_INDEX="1", PYTHONUTF8="1",
                       HTTP_PROXY="http://127.0.0.1:9", HTTPS_PROXY="http://127.0.0.1:9",
                       ALL_PROXY="http://127.0.0.1:9", NO_PROXY="", http_proxy="http://127.0.0.1:9",
                       https_proxy="http://127.0.0.1:9", all_proxy="http://127.0.0.1:9", no_proxy="")
    return environment


def check_text(output: str):
    if "你好世界 日本語テスト 한국어 123" not in output:
        raise RuntimeError("consumer OCR text mismatch: " + output[:2000])


def verify_python_upstream(dist: Path, root: Path, versions: list[str], report: dict):
    work = root / "Python application"
    work.mkdir()
    shutil.copy2(ROOT / "testdata/CJK.png", work / "image.png")
    wheel = dist / f"oneocr_native-{PYTHON_VERSION}-py3-none-any.whl"
    report["python"] = {}
    for version in versions:
        venv = root / ("python-" + version)
        subprocess.run(["uv", "venv", "--python", version, "--seed", str(venv)], check=True, capture_output=True)
        python = venv / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
        env = isolated(root / ("python-data-" + version))
        for name in ("PIP_NO_INDEX", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy"):
            if name in os.environ: env[name] = os.environ[name]
            else: env.pop(name, None)
        run(python, "-m", "pip", "install", wheel, cwd=work, env=env)
        run(python, "-c", "import importlib.util; assert importlib.util.find_spec('onnxruntime') is None; from oneocr_native import OneOcrEngine", cwd=work, env=env)
        # Seed only the model before the tag exists. The installer must fetch
        # ORT itself from upstream, without a platform-specific OneOCR bundle.
        bootstrap = root / ("bootstrap-" + version)
        (bootstrap / "models").mkdir(parents=True)
        shutil.copy2(ROOT / "models/oneocr-cjk-en.ocrpack", bootstrap / "models/oneocr-cjk-en.ocrpack")
        run(python, "-m", "oneocr_native", "install", cwd=bootstrap, env=env)
        check_text(run(python, "-m", "oneocr_native", "recognize", "image.png", cwd=work, env=env).stdout)
        check_text(run(python, "-c", "from oneocr_native import OneOcrEngine; e=OneOcrEngine(); print(e.recognize('image.png').text); assert e.detect('image.png').regions; assert e.recognize_line('image.png').text; e.close()", cwd=work, env=env).stdout)
        run(python, "-m", "oneocr_native", "install", "--offline", cwd=work, env=env)
        report["python"][version] = {"core_import_without_ort":True, "automatic_runtime_install":True, "cli":True, "api":True, "repeat":True}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dist",type=Path,required=True)
    parser.add_argument("--report",type=Path,required=True)
    parser.add_argument("--python-versions",default="3.11,3.12,3.13")
    parser.add_argument("--native-only",action="store_true")
    parser.add_argument("--python-only",action="store_true")
    parser.add_argument("--runtime-cache",type=Path)
    args = parser.parse_args()
    report={"platform":current_platform(),"version":VERSION}
    report["assets"] = {p.name:digest(p) for p in sorted(args.dist.iterdir()) if p.is_file() and (current_platform() in p.name or p.suffix == ".whl")}
    with tempfile.TemporaryDirectory(prefix="oneocr-release-consumer-") as temporary:
        root=Path(temporary)
        if not args.python_only:
            cache = args.runtime_cache or ROOT / "dist/reports" / current_platform() / "upstream/downloads"
            subprocess.run([sys.executable, str(ROOT / "scripts/verify_go.py"), "--dist", str(args.dist.resolve()), "--runtime-cache", str(cache), "--report", str(root / "go.json")], check=True)
            report["go"] = json.loads((root / "go.json").read_text(encoding="utf-8"))
            if current_platform().startswith("linux"):
                from verify_linux import verify
                report["linux"] = verify(args.dist.resolve(), cache.resolve())
        if not args.native_only:
            verify_python_upstream(args.dist.resolve(),root,args.python_versions.split(","),report)
    args.report.parent.mkdir(parents=True,exist_ok=True)
    args.report.write_text(json.dumps(report,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
    print(json.dumps(report,ensure_ascii=False))
if __name__ == "__main__":
    main()
