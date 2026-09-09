#!/usr/bin/env python3
"""Verify Release consumers from clean directories with networking disabled."""
from __future__ import annotations
import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path
from runtime_assets import current_platform, digest
from version import ROOT, VERSION, PYTHON_VERSION


def run(*command, cwd: Path, env: dict, success: bool = True):
    result = subprocess.run([str(x) for x in command], cwd=cwd, env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if success and result.returncode:
        raise RuntimeError(f"{command[0]} failed ({result.returncode}):\n{result.stdout[-4000:]}\n{result.stderr[-8000:]}")
    if not success and result.returncode == 0:
        raise RuntimeError("invalid installation unexpectedly succeeded")
    return result


def extract(archive: Path, directory: Path) -> Path:
    directory.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(archive) as source:
        for entry in source.infolist():
            if Path(entry.filename).is_absolute() or ".." in Path(entry.filename).parts or "\\" in entry.filename:
                raise RuntimeError("unsafe distribution entry")
            path = Path(source.extract(entry, directory))
            mode = (entry.external_attr >> 16) & 0o777
            if mode and path.is_file():
                path.chmod(mode)
    roots = [p for p in directory.iterdir() if p.is_dir()]
    if len(roots) != 1:
        raise RuntimeError("distribution must have one root directory")
    return roots[0]


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


def verify_native(dist: Path, root: Path, report: dict):
    target = current_platform()
    sdk = extract(dist / f"oneocr-sdk-{target}-{VERSION}.zip", root / "complete SDK 含空格")
    core = extract(dist / f"oneocr-core-{target}-{VERSION}.zip", root / "core SDK")
    work = root / "application"
    work.mkdir()
    image = work / "中文 图片.png"
    shutil.copy2(ROOT / "testdata/CJK.png", image)
    extension = ".exe" if os.name == "nt" else ""
    cli = sdk / "bin" / f"oneocr{extension}"
    env = isolated(root / "native")
    check_text(run(cli,"recognize",image,cwd=work,env=env).stdout)
    check_text(run(sdk / "bin" / f"oneocr-cpp{extension}",image,cwd=work,env=env).stdout)
    report["complete_cli_cpp"] = True
    cpp = root / "C++ application"
    shutil.copytree(sdk / "examples/cpp",cpp)
    build = cpp / "build"
    run("cmake","-S",cpp,"-B",build,f"-DCMAKE_PREFIX_PATH={sdk}","-DCMAKE_BUILD_TYPE=Release",cwd=work,env=env)
    run("cmake","--build",build,"--config","Release","--parallel","2",cwd=work,env=env)
    executable_dir = build / "Release" if (build / "Release").is_dir() else build
    portable = root / "portable application"
    portable.mkdir()
    copy_names = [f"oneocr-example{extension}","lib","models","licenses"]
    copy_names.extend(p.name for p in executable_dir.glob("*.dll"))
    for name in copy_names:
        source = executable_dir / name
        if source.is_dir(): shutil.copytree(source,portable / name)
        elif source.is_file(): shutil.copy2(source,portable / name)
    moved = sdk.with_name("SDK moved after build")
    sdk.rename(moved)
    try:
        check_text(run(portable / f"oneocr-example{extension}",image,cwd=work,env=env).stdout)
    finally:
        moved.rename(sdk)
    report["external_cmake_portable"] = True
    c_app = root / "C application"
    c_app.mkdir()
    (c_app / "CMakeLists.txt").write_text('cmake_minimum_required(VERSION 3.22)\nproject(oneocr_c_consumer LANGUAGES C)\nfind_package(OneOCR CONFIG REQUIRED)\nadd_executable(oneocr-c main.c)\ntarget_link_libraries(oneocr-c PRIVATE OneOCR::oneocr)\noneocr_copy_dependencies(oneocr-c)\n',encoding="utf-8")
    (c_app / "main.c").write_text('''#include "oneocr.h"
#include <stdio.h>
int main(void) {
    char *error = NULL;
    uint64_t engine = OneOCROpen(NULL, &error);
    if (!engine) { if (error) fputs(error, stderr); OneOCRFree(error); return 1; }
    OneOCRInput input = {0}; input.kind = ONEOCR_FILE; input.path = "image.png";
    char *result = OneOCRRecognize(engine, &input, NULL, &error);
    if (!result) { if (error) fputs(error, stderr); OneOCRFree(error); OneOCRClose(engine, NULL); return 2; }
    puts(result); OneOCRFree(result);
    if (OneOCRClose(engine, &error)) { OneOCRFree(error); return 3; }
    return 0;
}
''',encoding="utf-8")
    c_build = c_app / "build"
    run("cmake","-S",c_app,"-B",c_build,f"-DCMAKE_PREFIX_PATH={sdk}","-DCMAKE_BUILD_TYPE=Release",cwd=work,env=env)
    run("cmake","--build",c_build,"--config","Release",cwd=work,env=env)
    c_bin = c_build / "Release" if (c_build / "Release").is_dir() else c_build
    c_portable = root / "portable C application"
    c_portable.mkdir()
    for name in [f"oneocr-c{extension}","lib","models","licenses", *[p.name for p in c_bin.glob("*.dll")]]:
        source = c_bin / name
        if source.is_dir(): shutil.copytree(source,c_portable / name)
        elif source.is_file(): shutil.copy2(source,c_portable / name)
    shutil.copy2(image,work / "image.png")
    sdk.rename(moved)
    try:
        check_text(run(c_portable / f"oneocr-c{extension}",cwd=work,env=env).stdout)
    finally:
        moved.rename(sdk)
    report["external_c_portable"] = True
    core_cli = core / "bin" / f"oneocr{extension}"
    core_env = isolated(root / "core")
    run(core_cli,"install","--source",dist,"--offline",cwd=work,env=core_env)
    check_text(run(core_cli,"recognize",image,cwd=work,env=core_env).stdout)
    # Repeat without network or a release source; use the existing installation.
    run(core_cli,"install","--offline",cwd=work,env=core_env)
    report["offline_core_install_repeat"] = True
    corrupt = root / "corrupt Release"
    corrupt.mkdir()
    shutil.copy2(dist / "SHA256SUMS",corrupt / "SHA256SUMS")
    (corrupt / "release-manifest.json").write_text("{}\n",encoding="utf-8")
    bad_env = isolated(root / "bad")
    output = run(core_cli,"install","--source",corrupt,"--offline",cwd=work,env=bad_env,success=False)
    if "checksum" not in output.stderr or Path(bad_env["ONEOCR_HOME"],"config.json").exists():
        raise RuntimeError("corrupt download did not fail atomically")
    report["corrupt_release_rejected"] = True


def verify_python(dist: Path, root: Path, versions: list[str], report: dict):
    target = current_platform()
    bundle = extract(dist / f"oneocr-python-{target}-{VERSION}.zip",root / "Python SDK 含空格")
    work = root / "Python application"
    work.mkdir()
    image = work / "image.png"
    shutil.copy2(ROOT / "testdata/CJK.png",image)
    report["python"] = {}
    for version in versions:
        venv = root / ("python-" + version)
        # Prepare the interpreter and pip before the offline consumer boundary.
        subprocess.run(["uv","venv","--python",version,"--seed",str(venv)],check=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        python = venv / ("Scripts/python.exe" if os.name=="nt" else "bin/python")
        env = isolated(root / ("python-data-"+version))
        wheel = bundle / f"oneocr_native-{PYTHON_VERSION}-py3-none-any.whl"
        run(python,"-m","pip","install","--no-index","--find-links",bundle / "wheelhouse" / version,wheel,cwd=work,env=env)
        run(python,"-c","import importlib.util; assert importlib.util.find_spec('onnxruntime') is None; from oneocr_native import OneOcrEngine",cwd=work,env=env)
        run(python,"-m","oneocr_native","install","-h",cwd=work,env=env)
        run(python,bundle / "installation.py",cwd=work,env=env)
        check_text(run(python,"-m","oneocr_native","recognize",image,cwd=work,env=env).stdout)
        check_text(run(python,"-c","from oneocr_native import OneOcrEngine; e=OneOcrEngine(); print(e.recognize('image.png').text); assert e.detect('image.png').regions; e.close()",cwd=work,env=env).stdout)
        run(python,"-m","oneocr_native","install","--offline",cwd=work,env=env)
        report["python"][version] = {"core_import_without_ort":True,"offline_install":True,"cli":True,"api":True,"repeat":True}


def verify_python_upstream(dist: Path, root: Path, versions: list[str], report: dict):
    work = root / "Python application"
    work.mkdir()
    shutil.copy2(ROOT / "testdata/CJK.png", work / "image.png")
    wheel = dist / f"oneocr_native-{PYTHON_VERSION}-py3-none-any.whl"
    report["python"] = {}
    for version in versions:
        venv = root / ("python-" + version)
        subprocess.run(["uv", "venv", "--python", version, "--seed", str(venv)], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
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
                verify_native(args.dist.resolve(),root,report)
        if not args.native_only:
            verify = verify_python if current_platform().startswith("linux") else verify_python_upstream
            verify(args.dist.resolve(),root,args.python_versions.split(","),report)
    args.report.parent.mkdir(parents=True,exist_ok=True)
    args.report.write_text(json.dumps(report,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
    print(json.dumps(report,ensure_ascii=False))
if __name__ == "__main__":
    main()
