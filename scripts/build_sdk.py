#!/usr/bin/env python3
"""Build distributable SDKs; Python is a build tool, not a runtime dependency."""

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

ROOT = Path(__file__).resolve().parents[1]
REPO = ROOT
VERSION = "0.1.0"


def run(*args: str | Path, cwd: Path | None = None, env: dict | None = None) -> None:
    subprocess.run([str(a) for a in args], cwd=cwd, env=env, check=True)


def digest(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for data in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(data)
    return h.hexdigest()


def copy(source: Path, destination: Path) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, destination)


def archive(directory: Path, filename: Path, include_root: bool = True) -> None:
    temporary = filename.with_suffix(filename.suffix + ".tmp")
    with zipfile.ZipFile(
        temporary, "w", zipfile.ZIP_DEFLATED, compresslevel=6
    ) as zipped:
        for path in sorted(directory.rglob("*")):
            if path.is_file():
                name = path.relative_to(directory.parent if include_root else directory)
                zipped.write(path, name.as_posix())
    temporary.replace(filename)


def licenses(destination: Path, ort_licenses: Path) -> None:
    copy(REPO / "LICENSE", destination / "OneOCR-LICENSE.txt")
    copy(ort_licenses / "LICENSE", destination / "ONNXRuntime-LICENSE.txt")
    copy(
        ort_licenses / "ThirdPartyNotices.txt",
        destination / "ONNXRuntime-ThirdPartyNotices.txt",
    )
    cache = Path(
        subprocess.check_output(["go", "env", "GOMODCACHE"], text=True).strip()
    )
    copy(
        cache / "github.com/yalue/onnxruntime_go@v1.35.0/LICENSE",
        destination / "onnxruntime_go-LICENSE.txt",
    )
    copy(
        cache / "golang.org/x/text@v0.28.0/LICENSE",
        destination / "golang-x-text-LICENSE.txt",
    )
    copy(
        cache / "golang.org/x/text@v0.28.0/PATENTS",
        destination / "golang-x-text-PATENTS.txt",
    )


def copy_go(destination: Path) -> None:
    # Explicit source roots: never recurse through models, Python, dist or caches.
    paths = [
        p
        for p in ROOT.iterdir()
        if p.is_file()
        and (p.suffix in {".go", ".json"} or p.name in {"go.mod", "go.sum"})
    ]
    for folder in ("cmd", "examples"):
        paths.extend(
            p
            for p in (ROOT / folder).rglob("*")
            if p.is_file() and p.suffix in {".go", ".json"}
        )
    for path in paths:
        copy(path, destination / path.relative_to(ROOT))
    copy(ROOT / "LICENSE", destination / "LICENSE")
    copy(ROOT / "THIRD_PARTY_NOTICES.md", destination / "THIRD_PARTY_NOTICES.md")
    copy(ROOT / "docs/GO.md", destination / "README.md")
    readme = destination / "README.md"
    readme.write_text(readme.read_text(encoding="utf-8").replace(
        "](STAGES.md)", "](docs/STAGES.md)"
    ).replace(
        "](../LICENSE)", "](LICENSE)"
    ).replace(
        "](../examples/stream/main.go)", "](examples/stream/main.go)"
    ), encoding="utf-8")
    copy(ROOT / "sdk/SDK.md", destination / "sdk/SDK.md")
    for name in ("PACK_FORMAT.md", "BUNDLE.md", "GO.md", "STAGES.md"):
        copy(ROOT / "docs" / name, destination / "docs" / name)


def copy_sdk_readme(destination: Path) -> None:
    (destination / "README.md").write_text(
        (ROOT / "sdk/SDK.md").read_text(encoding="utf-8").replace(
            "](../docs/STAGES.md)", "](STAGES.md)"
        ), encoding="utf-8"
    )
    copy(ROOT / "docs/STAGES.md", destination / "STAGES.md")


def publish_tree(stage: Path, target: Path) -> None:
    if target.exists():
        if target.is_symlink() or not (target / "SDK.json").is_file():
            raise RuntimeError(
                f"refusing to replace a directory not marked as an SDK: {target}"
            )
        shutil.rmtree(target)
    stage.rename(target)


def write_record(stage: Path, platform: str, extra: dict) -> None:
    files = [
        {
            "file": p.relative_to(stage).as_posix(),
            "bytes": p.stat().st_size,
            "sha256": digest(p),
        }
        for p in sorted(stage.rglob("*"))
        if p.is_file()
    ]
    (stage / "SDK.json").write_text(
        json.dumps(
            {
                "schema": "oneocr.sdk.v1",
                "version": VERSION,
                "platform": platform,
                "onnxruntime": "1.29.0",
                "models_included": False,
                "module": "github.com/shiyori/oneocr-native",
                **extra,
                "files": files,
            },
            indent=2,
        )
        + "\n"
    )


def build_desktop(args: argparse.Namespace) -> list[Path]:
    if args.ort_library is None:
        raise RuntimeError("--ort-library is required for the desktop SDK")
    goos, goarch = subprocess.check_output(
        ["go", "env", "GOOS", "GOARCH"], text=True
    ).split()
    if goos not in {"darwin", "linux"}:
        raise RuntimeError(
            "this packaging script currently builds desktop SDKs on macOS/Linux"
        )
    platform = f"{goos}-{goarch}"
    with tempfile.TemporaryDirectory(prefix=".desktop-sdk-", dir=args.output) as temp:
        stage = Path(temp) / f"oneocr-sdk-{platform}"
        stage.mkdir()
        environment = dict(os.environ, ONEOCR_NATIVE_OUTPUT=str(stage / "lib"))
        run(ROOT / "scripts/build_shared.sh", env=environment)
        library_name = "liboneocr.dylib" if goos == "darwin" else "liboneocr.so"
        runtime_name = (
            "libonnxruntime.dylib" if goos == "darwin" else "libonnxruntime.so"
        )
        copy(args.ort_library, stage / "lib" / runtime_name)
        copy(ROOT / "sdk/include/oneocr.h", stage / "include/oneocr.h")
        copy(ROOT / "sdk/include/oneocr.hpp", stage / "include/oneocr.hpp")
        copy(
            ROOT / "sdk/cmake/OneOCRConfig.cmake",
            stage / "lib/cmake/OneOCR/OneOCRConfig.cmake",
        )
        (stage / "bin").mkdir()
        run(
            "go",
            "build",
            "-trimpath",
            "-o",
            stage / "bin/oneocr",
            "./cmd/oneocr",
            cwd=ROOT,
        )
        build = Path(temp) / "cpp-build"
        loader_path = (
            "@executable_path/../lib" if goos == "darwin" else "$ORIGIN/../lib"
        )
        run(
            "cmake",
            "-S",
            ROOT / "sdk/cpp",
            "-B",
            build,
            f"-DONEOCR_LIBRARY={stage / 'lib' / library_name}",
            "-DCMAKE_BUILD_WITH_INSTALL_RPATH=ON",
            f"-DCMAKE_INSTALL_RPATH={loader_path}",
        )
        run("cmake", "--build", build, "--parallel", "2")
        copy(build / "oneocr-cpp", stage / "bin/oneocr-cpp")
        copy(ROOT / "sdk/cpp/main.cpp", stage / "examples/cpp/main.cpp")
        (stage / "examples/cpp/CMakeLists.txt").write_text(
            "cmake_minimum_required(VERSION 3.22)\nproject(oneocr_example LANGUAGES CXX)\n"
            "find_package(OneOCR CONFIG REQUIRED)\nadd_executable(oneocr-example main.cpp)\n"
            "target_link_libraries(oneocr-example PRIVATE OneOCR::oneocr)\n"
        )
        copy_go(stage / "go")
        licenses(stage / "licenses", args.ort_licenses)
        copy_sdk_readme(stage)
        copy(ROOT / "docs/PACK_FORMAT.md", stage / "PACK_FORMAT.md")
        write_record(
            stage,
            platform,
            {"languages": ["C", "C++17", "Go"], "runtime_included": True},
        )
        target = args.output / stage.name
        publish_tree(stage, target)
    sdk_zip = args.output / f"{target.name}-{VERSION}.zip"
    archive(target, sdk_zip)
    # Also provide the Go module as a standalone source distribution.
    with tempfile.TemporaryDirectory(prefix=".go-sdk-", dir=args.output) as temp:
        module = Path(temp) / "oneocr-go-sdk"
        copy_go(module)
        licenses(module / "licenses", args.ort_licenses)
        copy(ROOT / "docs/PACK_FORMAT.md", module / "PACK_FORMAT.md")
        go_zip = args.output / f"oneocr-go-sdk-{VERSION}.zip"
        archive(module, go_zip)
    return [sdk_zip, go_zip]


def build_android(args: argparse.Namespace) -> list[Path]:
    if (
        args.ort_android_aar is None
        or args.android_sdk is None
        or args.android_ndk is None
    ):
        raise RuntimeError(
            "Android requires --ort-android-aar, --android-sdk and --android-ndk"
        )
    native = args.output / "android-native"
    environment = dict(
        os.environ,
        ANDROID_NDK_HOME=str(args.android_ndk),
        ONEOCR_ANDROID_OUTPUT=str(native),
    )
    for abi in ("arm64-v8a", "x86_64"):
        run(ROOT / "sdk/android/build-native.sh", abi, env=environment)
    android_jar = args.android_sdk / "platforms" / args.android_platform / "android.jar"
    with tempfile.TemporaryDirectory(prefix=".android-sdk-", dir=args.output) as temp:
        temp = Path(temp)
        stage = temp / "aar"
        stage.mkdir()
        classes = temp / "classes"
        classes.mkdir()
        java_sources = sorted((ROOT / "sdk/android/java").rglob("*.java"))
        run(
            "javac", "--release", "17", "-cp", android_jar, "-d", classes, *java_sources
        )
        run("jar", "--create", "--file", stage / "classes.jar", "-C", classes, ".")
        with zipfile.ZipFile(args.ort_android_aar) as upstream:
            for abi in ("arm64-v8a", "x86_64"):
                for name in ("liboneocr.so", "liboneocr_jni.so"):
                    copy(native / abi / name, stage / "jni" / abi / name)
                target = stage / "jni" / abi / "libonnxruntime.so"
                target.write_bytes(upstream.read(f"jni/{abi}/libonnxruntime.so"))
        (stage / "AndroidManifest.xml").write_text(
            '<?xml version="1.0" encoding="utf-8"?>\n'
            '<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="dev.oneocr">\n'
            '  <uses-sdk android:minSdkVersion="26" />\n</manifest>\n'
        )
        (stage / "R.txt").write_text("")
        (stage / "proguard.txt").write_text("-keep class dev.oneocr.OneOcr { *; }\n")
        licenses(stage / "META-INF/oneocr/licenses", args.ort_licenses)
        aar = args.output / f"oneocr-android-{VERSION}.aar"
        archive(stage, aar, include_root=False)
        # Compile a consumer against the actual classes.jar, not the source tree.
        run(
            "javac",
            "--release",
            "17",
            "-cp",
            os.pathsep.join([str(android_jar), str(stage / "classes.jar")]),
            "-d",
            temp / "consumer",
            ROOT / "sdk/android/sdk-example/Example.java",
        )
        kit = temp / "oneocr-android-sdk"
        copy(aar, kit / "libs" / aar.name)
        licenses(kit / "licenses", args.ort_licenses)
        copy_sdk_readme(kit)
        copy(ROOT / "sdk/android/sdk-example/Example.java", kit / "Example.java")
        copy(ROOT / "docs/PACK_FORMAT.md", kit / "PACK_FORMAT.md")
        write_record(
            kit,
            "android",
            {
                "abis": ["arm64-v8a", "x86_64"],
                "min_sdk": 26,
                "runtime_included": True,
                "upstream_aar_sha256": digest(args.ort_android_aar),
            },
        )
        kit_zip = args.output / f"oneocr-android-sdk-{VERSION}.zip"
        archive(kit, kit_zip)
    return [aar, kit_zip]


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--target", choices=("desktop", "android", "all"), default="all"
    )
    parser.add_argument("--output", type=Path, default=ROOT / "dist")
    parser.add_argument("--ort-library", type=Path)
    parser.add_argument("--ort-android-aar", type=Path)
    parser.add_argument(
        "--ort-licenses",
        type=Path,
        required=True,
        help="directory with ORT LICENSE and ThirdPartyNotices.txt",
    )
    parser.add_argument(
        "--android-sdk", type=Path, default=os.environ.get("ANDROID_HOME")
    )
    parser.add_argument(
        "--android-ndk", type=Path, default=os.environ.get("ANDROID_NDK_HOME")
    )
    parser.add_argument("--android-platform", default="android-36")
    args = parser.parse_args()
    for key, value in vars(args).items():
        if isinstance(value, Path):
            setattr(args, key, value.resolve())
    required = [
        args.ort_licenses / "LICENSE",
        args.ort_licenses / "ThirdPartyNotices.txt",
    ]
    if args.target in {"desktop", "all"} and args.ort_library is not None:
        required.append(args.ort_library)
    if args.target in {"android", "all"} and args.ort_android_aar is not None:
        required.append(args.ort_android_aar)
    for source in required:
        if not source.is_file():
            parser.error(f"required input is missing: {source}")
    args.output.mkdir(parents=True, exist_ok=True)
    outputs = []
    if args.target in {"desktop", "all"}:
        outputs += build_desktop(args)
    if args.target in {"android", "all"}:
        outputs += build_android(args)
    for path in outputs:
        print(
            json.dumps(
                {
                    "file": str(path),
                    "bytes": path.stat().st_size,
                    "sha256": digest(path),
                }
            )
        )


if __name__ == "__main__":
    main()
