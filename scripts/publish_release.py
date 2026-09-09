#!/usr/bin/env python3
"""Build records, audit and publication for exact assets from a tagged main commit."""
from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import tempfile
from pathlib import Path

from audit_release import audit, require
from release_manifest import create, expected_assets
from runtime_assets import digest
from version import PYTHON_VERSION, ROOT, TAG, VERSION

REPOSITORY = "shiyori/oneocr-native"
PLATFORMS = ("windows-amd64", "darwin-arm64", "linux-amd64", "linux-arm64")


def gh(*args):
    return subprocess.check_output(["gh", *map(str, args)], cwd=ROOT, text=True, encoding="utf-8")


def android_report(report, abi, host, assets):
    name = f"oneocr-android-{'core-' if host else ''}{VERSION}.aar"
    require(report.get("ok") is True and report.get("abi") == abi and report.get("host_version") == host, "Android report failed or has wrong ABI/runtime mode")
    require(report.get("runtime") == (host or "1.29.0") and report.get("hardware_test") == 1, "Android runtime/hardware test missing")
    require(report.get("aar_name") == name and report.get("aar_sha256") == digest(assets / name), "Android report does not cover the release AAR")
    require(set(report.get("input_forms", [])) >= {"encoded", "file", "bitmap", "RGBA-stride", "premultiplied", "P3"}, "Android input-form checks missing")
    require(report.get("text") == "你好世界 日本語テスト 한국어 123" and report.get("line_text"), "Android OCR mismatch")


def collect(run_id: str, commit: str, output: Path, reports: Path):
    require(re.fullmatch(r"[0-9]+", run_id), "invalid CI run id")
    metadata = json.loads(gh("api", f"repos/{REPOSITORY}/actions/runs/{run_id}"))
    require(metadata["head_sha"] == commit and metadata["status"] == "completed" and metadata["conclusion"] == "success", "build run is not a successful run of the tagged commit")
    require(metadata["path"] == ".github/workflows/release-build.yml", "wrong build workflow")
    require(metadata["head_branch"] == "main", "formal releases require a tested main commit")
    jobs = json.loads(gh("api", f"repos/{REPOSITORY}/actions/runs/{run_id}/jobs?per_page=100"))["jobs"]
    required = {"android", *("desktop-" + p for p in PLATFORMS)}
    require({j["name"] for j in jobs if j["conclusion"] == "success"} >= required, "required build/test job missing")
    output.mkdir(parents=True, exist_ok=True)
    reports.mkdir(parents=True, exist_ok=True)
    require(not any(output.iterdir()), "asset output directory must be empty")
    with tempfile.TemporaryDirectory(prefix="oneocr-ci-artifacts-") as temporary:
        temporary = Path(temporary)
        for platform in (*PLATFORMS, "android"):
            asset_dir = temporary / platform
            gh("run", "download", run_id, "--repo", REPOSITORY, "--name", "assets-" + platform, "--dir", asset_dir)
            for path in asset_dir.iterdir():
                require(path.is_file() and path.name in expected_assets(), "unexpected CI release artifact")
                destination = output / path.name
                if destination.exists():
                    require(digest(path) == digest(destination), "shared asset differs between platforms: " + path.name)
                else:
                    shutil.copy2(path, destination)
            gh("run", "download", run_id, "--repo", REPOSITORY, "--name", "reports-" + platform, "--dir", reports / platform)
    create(output)
    audit(output)
    def one(platform, name):
        paths = list((reports / platform).rglob(name))
        require(len(paths) == 1, f"missing or ambiguous {platform} {name}")
        return json.loads(paths[0].read_text(encoding="utf-8"))
    for platform in PLATFORMS:
        native = one(platform, "native.json")
        require(native.get("platform") == platform and native.get("passed") is True and native.get("go_race") is True and native.get("python_integration") is True and native.get("runtimes") == ["1.26.0", "1.29.0"] and native.get("go_comparisons") == 1008 and native.get("python_comparisons") == 504, "native baseline report incomplete")
        if platform in {"windows-amd64", "linux-amd64"}:
            gpu = one(platform, "gpu.json")
            require(gpu.get("platform") == platform and gpu.get("runtime") == "1.26.0" and all(gpu.get(k) is True for k in ("passed", "host_survived", "gpu_distribution_kept", "cpu_provider", "native_host_survived")), "existing GPU runtime compatibility checks missing")
        consumer = one(platform, "consumers.json")
        require(consumer.get("platform") == platform and consumer.get("version") == VERSION, "consumer platform/version mismatch")
        go = consumer.get("go", {})
        require(go.get("source_commit") == commit and go.get("platform") == platform and go.get("version") == VERSION, "Go consumer source/platform mismatch")
        for check in ("passed", "go_get", "library_install", "go_install", "cli_install", "empty_module_caches", "no_sdk", "no_replace", "host_binding_unchanged"):
            require(go.get(check) is True, "missing Go integration check: " + check)
        if platform.startswith("linux"):
            for check in ("complete_cli_cpp", "external_cmake_portable", "external_c_portable", "offline_core_install_repeat", "corrupt_release_rejected"):
                require(consumer.get(check) is True, "missing Linux consumer check: " + check)
        for version in ("3.11", "3.12", "3.13"):
            checks = consumer.get("python", {}).get(version, {})
            preparation = "offline_install" if platform.startswith("linux") else "automatic_runtime_install"
            require(all(checks.get(k) is True for k in ("core_import_without_ort", preparation, "cli", "api", "repeat")), "Python consumer checks incomplete")
        needed = {f"{prefix}-{platform}-{VERSION}.zip" for prefix in ("oneocr-sdk", "oneocr-core", "oneocr-python")} if platform.startswith("linux") else set()
        needed.add(f"oneocr_native-{PYTHON_VERSION}-py3-none-any.whl")
        require(needed <= consumer.get("assets", {}).keys(), "consumer asset hashes missing")
        for name, checksum in consumer["assets"].items():
            require(name in expected_assets() and digest(output / name) == checksum, "consumer tested a different artifact")
        require(one(platform, "audit.json").get("passed") is True, "platform archive audit missing")
    for name, host in (("full.json", None), ("host126.json", "1.26.0"), ("host129.json", "1.29.0")):
        android_report(one("android", name), "x86_64", host, output)
    require(one("android", "audit.json").get("passed") is True, "Android archive audit missing")


BUILD_PLATFORMS = ("linux-amd64", "linux-arm64", "android")
RECORD = "build-record.json"


def current_commit():
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()


def record(directory: Path, platform: str):
    require(platform in BUILD_PLATFORMS, "unsupported release build platform")
    create(directory, partial=True)
    audit(directory, partial=True)
    names = {p.name for p in directory.iterdir() if p.is_file() and not p.name.startswith(".")} - {"release-manifest.json", "SHA256SUMS"}
    require(all(expected_assets()[name][1] in {platform, ""} for name in names), "wrong platform asset")
    (directory / RECORD).write_text(json.dumps({
        "schema": "oneocr.build.v1", "version": VERSION, "commit": current_commit(), "platform": platform,
        "assets": {name: digest(directory / name) for name in sorted(names)},
    }, indent=2) + "\n", encoding="utf-8")


def assemble(packages: Path, output: Path):
    commit = current_commit()
    require({p.name for p in packages.iterdir()} == {"release-assets-" + p for p in BUILD_PLATFORMS}, "missing or unexpected build artifact")
    output.mkdir(parents=True, exist_ok=True)
    require(not any(output.iterdir()), "asset output directory must be empty")
    for platform in BUILD_PLATFORMS:
        source = packages / ("release-assets-" + platform)
        receipt = json.loads((source / RECORD).read_text(encoding="utf-8"))
        require(receipt.get("schema") == "oneocr.build.v1" and receipt.get("commit") == commit and receipt.get("version") == VERSION and receipt.get("platform") == platform, "build record does not match tagged source/platform")
        assets = receipt.get("assets", {})
        require(isinstance(assets, dict) and assets, "empty build record")
        require({p.name for p in source.iterdir()} == set(assets) | {RECORD, "release-manifest.json", "SHA256SUMS"}, "build record omits artifact files")
        for name, checksum in assets.items():
            require(name in expected_assets() and expected_assets()[name][1] in {platform, ""}, "unexpected build asset")
            require(digest(source / name) == checksum, "build asset checksum mismatch")
            destination = output / name
            if destination.exists():
                require(digest(destination) == checksum, "shared asset differs between platforms: " + name)
            else:
                shutil.copy2(source / name, destination)
    create(output)
    print(json.dumps(audit(output)))


def publish(assets: Path):
    commit = current_commit()
    tagged = subprocess.check_output(["git", "rev-parse", TAG + "^{commit}"], cwd=ROOT, text=True).strip()
    require(tagged == commit, "release tag does not match checked-out source")
    require(subprocess.run(["git", "merge-base", "--is-ancestor", commit, "origin/main"], cwd=ROOT, check=False).returncode == 0, "formal releases must come from main")
    require(re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", VERSION), "Latest requires a stable version")
    runs = json.loads(gh("api", f"repos/{REPOSITORY}/actions/runs?head_sha={commit}&per_page=100"))["workflow_runs"]
    require(any(run["head_sha"] == commit and run["path"] == ".github/workflows/ci.yml" and run["conclusion"] == "success" for run in runs), "regular CI has not passed for the tagged commit")
    audit(assets)
    with tempfile.TemporaryDirectory(prefix="oneocr-publish-") as temporary:
        body = Path(temporary) / "release.md"
        body.write_text(f"""OneOCR {VERSION}

Offline Chinese, Japanese, Korean and English OCR for Go, C, C++, Python and Android.

- Go integration through `go get` or `go install`, without a desktop SDK download.
- Android full/Core AARs, a universal Python wheel, and optional Linux packages.
- No Windows/macOS platform-specific artifacts; missing dependencies are prepared on demand.
- Existing compatible ONNX Runtime 1.26+ can be reused; managed bundles include 1.29.0.
- Windows x64, macOS ARM64, Linux x64/ARM64, Android ARM64/x86_64; Python 3.11–3.13.
- Three operations: recognize, detect and recognize-line. File, memory and native pixel inputs are documented per language.

[简体中文](https://github.com/{REPOSITORY}/blob/{TAG}/README.md) · [English](https://github.com/{REPOSITORY}/blob/{TAG}/README.en.md) · [日本語](https://github.com/{REPOSITORY}/blob/{TAG}/README.ja.md)

Release assets were built from the tagged main commit and checked for completeness, checksums, required libraries and licenses. Linux builds ran native ABI smoke tests. The regular Go/Python CI passed for this commit; extended compatibility and OCR comparison suites remain available as a separate manual workflow.

Verify downloads with `SHA256SUMS` and `release-manifest.json`. Code: AGPL-3.0-only. Review `Model-NOTICE.txt` for the separately provided model and bundled third-party notices.
""", encoding="utf-8")
        existing = subprocess.run(["gh", "release", "view", TAG, "--repo", REPOSITORY, "--json", "isDraft,tagName"], text=True, capture_output=True, check=False)
        names = sorted([*expected_assets(), "release-manifest.json", "SHA256SUMS"])
        if existing.returncode == 0 and not json.loads(existing.stdout)["isDraft"]:
            verify_uploaded(assets, names)
            gh("release", "edit", TAG, "--repo", REPOSITORY, "--prerelease=false", "--latest")
            print(gh("release", "view", TAG, "--repo", REPOSITORY, "--json", "url,isDraft"))
            return
        if existing.returncode != 0:
            gh("release", "create", TAG, "--repo", REPOSITORY, "--verify-tag", "--draft", "--title", f"OneOCR {VERSION}", "--notes-file", body)
        gh("release", "upload", TAG, "--repo", REPOSITORY, *[assets / n for n in names], "--clobber")
        verify_uploaded(assets, names)
        gh("release", "edit", TAG, "--repo", REPOSITORY, "--draft=false", "--prerelease=false", "--latest", "--notes-file", body)
        print(gh("release", "view", TAG, "--repo", REPOSITORY, "--json", "url,isDraft"))


def verify_uploaded(assets, names):
    remote = json.loads(gh("api", f"repos/{REPOSITORY}/releases/tags/{TAG}"))
    require({a["name"] for a in remote["assets"]} == set(names), "uploaded asset set differs")
    for asset in remote["assets"]:
        require(asset.get("digest") == "sha256:" + digest(assets / asset["name"]) and asset["size"] == (assets / asset["name"]).stat().st_size, "uploaded checksum differs")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    gather = commands.add_parser("collect", help="collect a manually completed full validation run")
    gather.add_argument("--run-id", required=True)
    gather.add_argument("--output", type=Path, required=True)
    gather.add_argument("--reports", type=Path, required=True)
    recording = commands.add_parser("record")
    recording.add_argument("--dist", type=Path, required=True)
    recording.add_argument("--platform", choices=BUILD_PLATFORMS, required=True)
    assembling = commands.add_parser("assemble")
    assembling.add_argument("--packages", type=Path, required=True)
    assembling.add_argument("--output", type=Path, required=True)
    publishing = commands.add_parser("publish")
    publishing.add_argument("--dist", type=Path, required=True)
    args = parser.parse_args()
    if args.command == "collect":
        collect(args.run_id, current_commit(), args.output, args.reports)
    elif args.command == "record":
        record(args.dist, args.platform)
    elif args.command == "assemble":
        assemble(args.packages, args.output)
    else:
        publish(args.dist.resolve())


if __name__ == "__main__":
    main()
