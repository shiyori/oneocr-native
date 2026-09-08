#!/usr/bin/env python3
"""Relocate and exercise local SDK artifacts; emit portable validation evidence."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import posixpath
import re
import shutil
import subprocess
import tempfile
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def run(*args, cwd=None, env=None):
    result = subprocess.run(
        [str(a) for a in args],
        cwd=cwd,
        env=env,
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--android-ndk", type=Path)
    args = parser.parse_args()
    report = {
        "schema": "oneocr.distribution.validation.v1",
        "host": "darwin-arm64",
        "checks": {},
    }
    checks = report["checks"]
    dist = ROOT / "dist"
    with tempfile.TemporaryDirectory(prefix="oneocr-consumer-") as temporary:
        temp = Path(temporary)
        with zipfile.ZipFile(dist / "oneocr-sdk-darwin-arm64-0.1.0.zip") as archive:
            archive.extractall(temp)
        sdk = temp / "oneocr-sdk-darwin-arm64"
        for path in (sdk / "bin").iterdir():
            path.chmod(0o755)
        model = temp / "oneocr-cjk-en.ocrpack"
        shutil.copy2(ROOT / "models" / model.name, model)
        image = temp / "image.png"
        shutil.copy2(ROOT / "testdata/CJK.png", image)
        expected = "你好世界 日本語テスト 한국어 123"
        environment = dict(os.environ)
        for key in (
            "ONEOCR_RUNTIME",
            "DYLD_LIBRARY_PATH",
            "LD_LIBRARY_PATH",
            "PYTHONPATH",
        ):
            environment.pop(key, None)
        actual = json.loads(
            run(sdk / "bin/oneocr-cpp", model, image, cwd=temp, env=environment)
        )
        assert actual["text"] == expected
        checks["relocated_cpp_binary"] = "passed"
        consumer = temp / "cpp"
        shutil.copytree(sdk / "examples/cpp", consumer)
        build = temp / "cpp-build"
        run("cmake", "-S", consumer, "-B", build, f"-DCMAKE_PREFIX_PATH={sdk}")
        run("cmake", "--build", build, "--parallel", "2")
        actual = json.loads(
            run(build / "oneocr-example", model, image, cwd=temp, env=environment)
        )
        assert actual["text"] == expected
        checks["external_cmake_target"] = "passed"
        environment["ONEOCR_HOME"] = str(temp / "installation")
        run(sdk / "bin/oneocr", "install", "--model", model, cwd=temp, env=environment)
        output = run(sdk / "bin/oneocr", "recognize", image, cwd=temp, env=environment)
        assert expected in output
        checks["relocated_cli_install_and_recognize"] = "passed"
        go = temp / "go-consumer"
        go.mkdir()
        run("go", "mod", "init", "example.org/consumer", cwd=go)
        run(
            "go",
            "mod",
            "edit",
            f"-replace=github.com/shiyori/oneocr-native={sdk / 'go'}",
            cwd=go,
        )
        run("go", "get", "github.com/shiyori/oneocr-native", cwd=go)
        (go / "main.go").write_text("""package main
import ("context"; "fmt"; "os"; oneocr "github.com/shiyori/oneocr-native")
func main(){ e,err:=oneocr.OpenInstalled("");if err!=nil{panic(err)};defer e.Close()
r,err:=e.RecognizeFile(context.Background(),os.Args[1],oneocr.Options{});if err!=nil{panic(err)};fmt.Println(r.Text)}
""")
        assert expected in run("go", "run", ".", image, cwd=go, env=environment)
        checks["external_go_source_sdk"] = "passed"
        # A fresh Python consumer receives only the wheel, model and input image.
        python_root = temp / "python-consumer"
        python_root.mkdir()
        shutil.copy2(model, python_root / model.name)
        shutil.copy2(image, python_root / image.name)
        wheel = dist / "oneocr_native-0.1.0-py3-none-any.whl"
        run("uv", "venv", "--python", "3.13", python_root / ".venv")
        executable = python_root / ".venv/bin/python"
        run("uv", "pip", "install", "--python", executable, wheel)
        clean = dict(environment)
        clean.pop("ONEOCR_HOME", None)
        script = """import pathlib, oneocr_native
from oneocr_native import OneOcrEngine
assert '.venv' in oneocr_native.__file__
assert not list(pathlib.Path('.').rglob('liboneocr.*'))
with OneOcrEngine('oneocr-cjk-en.ocrpack') as e:
 print(e.recognize('image.png').text)
assert e.prepared._file.closed
"""
        assert expected in run(
            executable, "-I", "-c", script, cwd=python_root, env=clean
        )
        checks["isolated_python_wheel_without_go_library"] = "passed"
        checks["recognized_text"] = expected
        if args.android_ndk:
            with zipfile.ZipFile(dist / "oneocr-android-0.1.0.aar") as aar:
                aar.extractall(temp / "aar")
            readelf = (
                args.android_ndk
                / "toolchains/llvm/prebuilt/darwin-x86_64/bin/llvm-readelf"
            )
            natives = []
            for lib in sorted((temp / "aar/jni").rglob("*.so")):
                headers = run(readelf, "-lW", lib)
                loads = [
                    line.split()
                    for line in headers.splitlines()
                    if line.strip().startswith("LOAD ")
                ]
                assert loads and all(int(line[-1], 16) >= 16384 for line in loads)
                dynamic = run(readelf, "-d", lib)
                needed = [
                    line.split("[")[1].split("]")[0]
                    for line in dynamic.splitlines()
                    if "(NEEDED)" in line
                ]
                assert all("/" not in name for name in needed)
                natives.append(
                    {
                        "file": lib.relative_to(temp / "aar").as_posix(),
                        "load_alignment_min": min(int(line[-1], 16) for line in loads),
                        "needed": needed,
                    }
                )
            checks["android_elf"] = natives
    # Archives must not contain build outputs or private machine paths in docs.
    inspected = []
    for artifact in sorted(dist.glob("*")):
        if artifact.suffix not in {".zip", ".aar", ".whl"}:
            continue
        with zipfile.ZipFile(artifact) as zipped:
            names = zipped.namelist()
            assert not any(
                any(
                    part in {".venv", "__pycache__", ".git", ".cache", "CMakeFiles"}
                    for part in Path(name).parts
                )
                for name in names
            )
            assert any("LICENSE" in n for n in names), artifact.name
            for name in names:
                if name.endswith((".md", ".go", ".py", "go.mod")):
                    value = zipped.read(name).decode("utf-8")
                    assert "/Users/shiyori" not in value, (artifact.name, name)
                    assert "ShiyoriAutoAgent" not in value, (artifact.name, name)
                    if name.endswith(".md"):
                        for link in re.findall(r"\]\(([^)]+)\)", value):
                            if ":" in link or link.startswith("#"):
                                continue
                            target = posixpath.normpath(
                                posixpath.join(
                                    posixpath.dirname(name), link.split("#")[0]
                                )
                            )
                            assert target in names or any(
                                n.startswith(target.rstrip("/") + "/") for n in names
                            ), (artifact.name, name, link)
            if "go-sdk" in artifact.name:
                assert not any(n.endswith((".ocrpack", ".dylib", ".so")) for n in names)
        inspected.append(
            {
                "file": artifact.name,
                "bytes": artifact.stat().st_size,
                "sha256": hashlib.sha256(artifact.read_bytes()).hexdigest(),
            }
        )
    report["artifacts"] = inspected
    report["limits"] = [
        "Android native/AAR/consumer compile only; no device OCR",
        "Windows/Linux desktop not executed on this host",
    ]
    output = ROOT / "validation/distributions.json"
    output.parent.mkdir(exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"checks": list(checks), "artifacts": len(inspected)}))


if __name__ == "__main__":
    main()
