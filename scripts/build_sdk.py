#!/usr/bin/env python3
"""Build complete and core SDKs for GitHub Releases."""
from __future__ import annotations
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import zipfile
from pathlib import Path
from version import ROOT, VERSION, TAG, RUNTIME_VERSION
from runtime_assets import current_platform, prepare


def run(*args: str | Path, cwd: Path | None = None, env: dict | None = None) -> None:
    subprocess.run([str(a) for a in args], cwd=cwd, env=env, check=True)


def digest(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def copy(source: Path, destination: Path) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, destination)


def add_zip_file(zipped: zipfile.ZipFile, path: Path, name: str) -> None:
    info = zipfile.ZipInfo(name, date_time=(2026, 1, 1, 0, 0, 0))
    info.create_system = 3
    info.compress_type = zipfile.ZIP_DEFLATED
    mode = 0o755 if "bin" in Path(name).parts or path.suffix == ".sh" else 0o644
    info.external_attr = (0o100000 | mode) << 16
    with path.open("rb") as source, zipped.open(info, "w", force_zip64=True) as target:
        shutil.copyfileobj(source, target)


def archive(directory: Path, filename: Path, include_root: bool = True) -> None:
    temporary = filename.with_suffix(filename.suffix + ".tmp")
    with zipfile.ZipFile(temporary, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as zipped:
        for path in sorted(directory.rglob("*")):
            if path.is_file():
                add_zip_file(zipped, path, path.relative_to(directory.parent if include_root else directory).as_posix())
    temporary.replace(filename)


def licenses(destination: Path, runtime_directory: Path | None = None) -> None:
    copy(ROOT / "LICENSE", destination / "OneOCR-LICENSE.txt")
    copy(ROOT / "internal/ort/ONNXRUNTIME-LICENSE", destination / "ONNXRuntime-Header-LICENSE.txt")
    copy(ROOT / "models/LICENSE", destination / "Model-NOTICE.txt")
    if runtime_directory is not None:
        copy(runtime_directory / "LICENSE", destination / "ONNXRuntime-LICENSE.txt")
        copy(runtime_directory / "ThirdPartyNotices.txt", destination / "ONNXRuntime-ThirdPartyNotices.txt")
    cache = Path(subprocess.check_output(["go", "env", "GOMODCACHE"], text=True).strip())
    for name in ("LICENSE", "PATENTS"):
        copy(cache / "golang.org/x/text@v0.28.0" / name, destination / f"golang-x-text-{name}.txt")


def copy_docs(destination: Path) -> None:
    for name in ("README.md", "README.en.md", "README.ja.md", "LICENSE", "THIRD_PARTY_NOTICES.md"):
        copy(ROOT / name, destination / name)
    for language in ("zh-CN", "en", "ja"):
        shutil.copytree(ROOT / "docs" / language, destination / "docs" / language, dirs_exist_ok=True)


def copy_go(destination: Path) -> None:
    paths = [p for p in ROOT.iterdir() if p.is_file() and (p.suffix in {".go", ".json"} or p.name in {"go.mod", "go.sum"})]
    for folder in ("cmd", "examples", "internal", "sdk/include"):
        paths.extend(p for p in (ROOT / folder).rglob("*") if p.is_file() and
                     (p.suffix in {".go", ".json", ".c", ".h", ".hpp"} or "LICENSE" in p.name))
    for path in paths:
        if not path.name.endswith("_test.go"):
            copy(path, destination / path.relative_to(ROOT))
    copy_docs(destination)


def go_proxy(source: Path, destination: Path) -> None:
    # Third-party modules keep their upstream Go checksums. The SDK itself is
    # integrated as a managed local module, so its source archive does not
    # impersonate the canonical checksum of the repository tag.
    output = subprocess.check_output(["go", "mod", "download", "-json", "all"], cwd=ROOT, text=True)
    decoder = json.JSONDecoder()
    while output.strip():
        output = output.lstrip()
        module, end = decoder.raw_decode(output)
        output = output[end:]
        if "Error" in module:
            raise RuntimeError(module["Error"])
        target = destination / module["Path"] / "@v"
        for field, suffix in (("GoMod", "mod"), ("Zip", "zip"), ("Info", "info")):
            copy(Path(module[field]), target / f"{module['Version']}.{suffix}")
        (target / "list").write_text(module["Version"] + "\n", encoding="utf-8", newline="\n")


def write_record(stage: Path, platform: str, **extra) -> None:
    files = [{"file": p.relative_to(stage).as_posix(), "bytes": p.stat().st_size, "sha256": digest(p)}
             for p in sorted(stage.rglob("*")) if p.is_file() and p.name != "SDK.json"]
    (stage / "SDK.json").write_text(json.dumps({"schema": "oneocr.sdk.v1", "version": VERSION,
        "platform": platform, "onnxruntime": RUNTIME_VERSION, "minimum_runtime_api": 26,
        "module": "github.com/shiyori/oneocr-native", **extra, "files": files}, indent=2) + "\n", encoding="utf-8")


def _native_library(stage: Path, target: str) -> Path:
    directory = stage / ("bin" if target.startswith("windows") else "lib")
    directory.mkdir(parents=True, exist_ok=True)
    name = "oneocr.dll" if target.startswith("windows") else "liboneocr.dylib" if target.startswith("darwin") else "liboneocr.so"
    library = directory / name
    flags = []
    if target.startswith("darwin"):
        flags = ["-ldflags=-extldflags=-Wl,-install_name,@rpath/liboneocr.dylib,-rpath,@loader_path"]
    elif target.startswith("linux"):
        flags = ["-ldflags=-extldflags=-Wl,-soname,liboneocr.so,-rpath,$ORIGIN"]
    else:
        flags = ["-ldflags=-extldflags=-static-libgcc"]
    run("go", "build", "-trimpath", "-buildmode=c-shared", *flags, "-o", library, "./cmd/oneocr-shared", cwd=ROOT, env=dict(os.environ, CGO_ENABLED="1"))
    # Ship the reviewed public header, not cgo's implementation header.
    generated = library.with_suffix(".h")
    if generated.exists():
        generated.unlink()
    if target.startswith("windows"):
        exports = ["OneOCROpen", "OneOCRRecognize", "OneOCRDetect", "OneOCRRecognizeLine", "OneOCRWarmup", "OneOCRDiagnostics", "OneOCRClose", "OneOCRFree"]
        definition = stage / "lib/oneocr.def"
        definition.parent.mkdir(parents=True, exist_ok=True)
        definition.write_text("LIBRARY oneocr.dll\nEXPORTS\n" + "\n".join(exports) + "\n", encoding="utf-8")
        librarian = shutil.which("lib.exe") or shutil.which("llvm-lib")
        if not librarian:
            raise RuntimeError("MSVC lib.exe or llvm-lib is needed to generate the C++ import library")
        run(librarian, f"/def:{definition}", "/machine:x64", f"/out:{stage / 'lib/oneocr.lib'}")
        return stage / "lib/oneocr.lib"
    return library


def build_desktop(args: argparse.Namespace) -> list[Path]:
    target = current_platform()
    runtime_directory = args.runtime_directory or prepare(target, args.output / ".upstream")
    executable = ".exe" if target.startswith("windows") else ""
    outputs = []
    with tempfile.TemporaryDirectory(prefix=".desktop-sdk-", dir=args.output) as temporary:
        temporary = Path(temporary)
        stage = temporary / f"oneocr-sdk-{target}"
        library = _native_library(stage, target)
        copy(ROOT / "sdk/include/oneocr.h", stage / "include/oneocr.h")
        copy(ROOT / "sdk/include/oneocr.hpp", stage / "include/oneocr.hpp")
        copy(ROOT / "sdk/cmake/OneOCRConfig.cmake", stage / "lib/cmake/OneOCR/OneOCRConfig.cmake")
        (stage / "bin").mkdir(exist_ok=True)
        run("go", "build", "-trimpath", "-o", stage / "bin" / f"oneocr{executable}", "./cmd/oneocr", cwd=ROOT)
        build = temporary / "cpp-build"
        options = [f"-DONEOCR_LIBRARY={library}", "-DCMAKE_BUILD_TYPE=Release", "-DBUILD_TESTING=ON"]
        if target.startswith("darwin"):
            options += ["-DCMAKE_BUILD_WITH_INSTALL_RPATH=ON", "-DCMAKE_INSTALL_RPATH=@executable_path/../lib"]
        elif target.startswith("linux"):
            options += ["-DCMAKE_BUILD_WITH_INSTALL_RPATH=ON", "-DCMAKE_INSTALL_RPATH=$ORIGIN/../lib"]
        run("cmake", "-S", ROOT / "sdk/cpp", "-B", build, *options)
        run("cmake", "--build", build, "--config", "Release", "--parallel", "2")
        cpp_directory = build / "Release" if (build / "Release").is_dir() else build
        test_env = dict(os.environ, ONEOCR_BUNDLE=str(ROOT / "models/oneocr-cjk-en.ocrpack"),
                        ONEOCR_RUNTIME=str(runtime_directory / ("onnxruntime.dll" if target.startswith("windows") else "libonnxruntime.dylib" if target.startswith("darwin") else "libonnxruntime.so")),
                        ONEOCR_LINE_IMAGE=str(ROOT / "testdata/Latin.png"))
        test_env["PATH"] = os.pathsep.join([str(stage / "bin"), str(runtime_directory), test_env.get("PATH", "")])
        run("ctest", "--test-dir", build, "-C", "Release", "--output-on-failure", env=test_env)
        copy(cpp_directory / f"oneocr-cpp{executable}", stage / "bin" / f"oneocr-cpp{executable}")
        copy(ROOT / "sdk/cpp/main.cpp", stage / "examples/cpp/main.cpp")
        (stage / "examples/cpp/CMakeLists.txt").write_text(
            "cmake_minimum_required(VERSION 3.22)\nproject(oneocr_example LANGUAGES CXX)\n"
            "find_package(OneOCR CONFIG REQUIRED)\nadd_executable(oneocr-example main.cpp)\n"
            "target_link_libraries(oneocr-example PRIVATE OneOCR::oneocr)\n"
            "oneocr_copy_dependencies(oneocr-example)\n"
            "if(MINGW)\n  target_link_options(oneocr-example PRIVATE -municode)\nendif()\n", encoding="utf-8")
        copy_docs(stage)
        licenses(stage / "licenses", runtime_directory)
        # Core keeps the same SDK layout and setup tool; only model/ORT are omitted.
        core = temporary / f"oneocr-core-{target}"
        shutil.copytree(stage, core)
        if target.startswith("windows"):
            for path in runtime_directory.glob("*.dll"):
                if "onnxruntime" not in path.name:
                    copy(path, core / "bin" / path.name)
                    copy(path, stage / "bin" / path.name)
        write_record(core, target, runtime_included=False, models_included=False, languages=["C", "C++17", "CLI"])
        core_zip = args.output / f"{core.name}-{VERSION}.zip"
        archive(core, core_zip); outputs.append(core_zip)
        for path in runtime_directory.iterdir():
            if path.is_file():
                copy(path, stage / "lib" / path.name)
        copy(ROOT / "models/oneocr-cjk-en.ocrpack", stage / "models/oneocr-cjk-en.ocrpack")
        copy(ROOT / "models/LICENSE", stage / "models/LICENSE")
        write_record(stage, target, runtime_included=True, models_included=True, languages=["C", "C++17", "CLI"])
        complete_zip = args.output / f"{stage.name}-{VERSION}.zip"
        archive(stage, complete_zip); outputs.append(complete_zip)
        runtime_zip = args.output / f"oneocr-runtime-{RUNTIME_VERSION}-{target}.zip"
        archive(runtime_directory, runtime_zip, include_root=False); outputs.append(runtime_zip)
    return outputs


def build_android(args: argparse.Namespace) -> list[Path]:
    if args.android_sdk is None or args.android_ndk is None:
        raise RuntimeError("Android SDK and NDK are required")
    upstream_aar = args.ort_android_aar or prepare("android", args.output / ".upstream")
    runtime_directory = args.runtime_directory or prepare(current_platform(), args.output / ".upstream")
    native = args.output / ".android-native"
    environment = dict(os.environ, ANDROID_NDK_HOME=str(args.android_ndk), ONEOCR_ANDROID_OUTPUT=str(native))
    for abi in ("arm64-v8a", "x86_64"):
        run(ROOT / "sdk/android/build-native.sh", abi, env=environment)
    android_jar = args.android_sdk / "platforms" / args.android_platform / "android.jar"
    outputs = []
    with tempfile.TemporaryDirectory(prefix=".android-sdk-", dir=args.output) as temporary:
        temporary = Path(temporary)
        stage = temporary / "aar"
        stage.mkdir()
        classes = temporary / "classes"
        classes.mkdir()
        run("javac", "--release", "17", "-cp", android_jar, "-d", classes, *sorted((ROOT / "sdk/android/java").rglob("*.java")))
        run("jar", "--create", "--file", stage / "classes.jar", "-C", classes, ".")
        for abi in ("arm64-v8a", "x86_64"):
            for name in ("liboneocr.so", "liboneocr_jni.so"):
                copy(native / abi / name, stage / "jni" / abi / name)
        (stage / "AndroidManifest.xml").write_text(
            '<?xml version="1.0" encoding="utf-8"?>\n<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="dev.oneocr"><uses-sdk android:minSdkVersion="26" /></manifest>\n', encoding="utf-8")
        (stage / "R.txt").write_text("", encoding="utf-8")
        (stage / "proguard.txt").write_text("-keep class dev.oneocr.OneOcr { *; }\n-keep class dev.oneocr.OneOcr$* { *; }\n", encoding="utf-8")
        licenses(stage / "META-INF/oneocr/licenses", runtime_directory)
        core = args.output / "oneocr-android-core.aar"
        archive(stage, core, include_root=False); outputs.append(core)
        runtime = temporary / "android-runtime"
        runtime.mkdir()
        with zipfile.ZipFile(upstream_aar) as upstream:
            for abi in ("arm64-v8a", "x86_64"):
                path = runtime / "jni" / abi / "libonnxruntime.so"
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(upstream.read(f"jni/{abi}/libonnxruntime.so"))
                copy(path, stage / "jni" / abi / path.name)
        copy(ROOT / "models/oneocr-cjk-en.ocrpack", stage / "assets/oneocr-cjk-en.ocrpack")
        complete = args.output / "oneocr-android.aar"
        archive(stage, complete, include_root=False); outputs.append(complete)
        run("javac", "--release", "17", "-cp", os.pathsep.join([str(android_jar), str(stage / "classes.jar")]), "-d", temporary / "consumer", ROOT / "sdk/android/sdk-example/Example.java")
    return outputs


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", choices=("desktop", "android", "all"), default="desktop")
    parser.add_argument("--output", type=Path, default=ROOT / "dist/release")
    parser.add_argument("--runtime-directory", type=Path, help="prepared upstream files and runtime.json")
    parser.add_argument("--ort-android-aar", type=Path)
    parser.add_argument("--android-sdk", type=Path, default=os.environ.get("ANDROID_HOME"))
    parser.add_argument("--android-ndk", type=Path, default=os.environ.get("ANDROID_NDK_HOME"))
    parser.add_argument("--android-platform", default="android-36")
    args = parser.parse_args()
    for key, value in vars(args).items():
        if isinstance(value, Path):
            setattr(args, key, value.resolve())
    args.output.mkdir(parents=True, exist_ok=True)
    run("go", "mod", "download", cwd=ROOT)
    outputs = []
    if args.target in {"desktop", "all"}:
        outputs += build_desktop(args)
    if args.target in {"android", "all"}:
        outputs += build_android(args)
    for path in outputs:
        print(json.dumps({"file": str(path), "bytes": path.stat().st_size, "sha256": digest(path)}))
if __name__ == "__main__":
    main()
