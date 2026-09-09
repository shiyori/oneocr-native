#!/usr/bin/env python3
"""Publish only the exact successful CI artifacts named by an annotated-tag receipt."""
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
from version import ROOT, VERSION, TAG

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
        consumer = one(platform, "consumers.json")
        require(consumer.get("platform") == platform and consumer.get("version") == VERSION, "consumer platform/version mismatch")
        for check in ("complete_cli_cpp", "external_cmake_portable", "offline_go_no_cache", "offline_core_install_repeat", "corrupt_release_rejected"):
            require(consumer.get(check) is True, "missing consumer check: " + check)
        for version in ("3.11", "3.12", "3.13"):
            checks = consumer.get("python", {}).get(version, {})
            require(all(checks.get(k) is True for k in ("core_import_without_ort", "offline_install", "cli", "api", "repeat")), "Python offline consumer checks incomplete")
        needed = {f"{prefix}-{platform}-{VERSION}.zip" for prefix in ("oneocr-sdk", "oneocr-core", "oneocr-python")}
        require(needed <= consumer.get("assets", {}).keys(), "consumer asset hashes missing")
        for name, checksum in consumer["assets"].items():
            require(name in expected_assets() and digest(output / name) == checksum, "consumer tested a different artifact")
        require(one(platform, "audit.json").get("passed") is True, "platform archive audit missing")
    for name, host in (("full.json", None), ("host126.json", "1.26.0"), ("host129.json", "1.29.0")):
        android_report(one("android", name), "x86_64", host, output)
    require(one("android", "audit.json").get("passed") is True, "Android archive audit missing")


def publish(receipt):
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    require(receipt.get("schema") == "oneocr.release-receipt.v1" and receipt.get("version") == VERSION and receipt.get("commit") == commit, "tag receipt version/commit mismatch")
    with tempfile.TemporaryDirectory(prefix="oneocr-publish-") as temporary:
        temporary = Path(temporary)
        assets = temporary / "assets"
        collect(str(receipt["build_run_id"]), commit, assets, temporary / "reports")
        arm = receipt.get("android_arm64", [])
        require(isinstance(arm, list) and len(arm) == 3, "three actual ARM64 reports are required")
        for report, host in zip(arm, (None, "1.26.0", "1.29.0"), strict=True):
            android_report(report, "arm64-v8a", host, assets)
        body = temporary / "release.md"
        body.write_text(f"""OneOCR {VERSION} prerelease

Offline Chinese, Japanese, Korean and English OCR for Go, C, C++, Python and Android.

- Full offline SDKs and Core SDKs without bundled models or ONNX Runtime.
- Existing compatible ONNX Runtime 1.26+ can be reused; managed bundles include 1.29.0.
- Windows x64, macOS ARM64, Linux x64/ARM64, Android ARM64/x86_64; Python 3.11–3.13.
- Three operations: recognize, detect and recognize-line. File, memory and native pixel inputs are documented per language.

[简体中文](https://github.com/{REPOSITORY}/blob/{TAG}/README.md) · [English](https://github.com/{REPOSITORY}/blob/{TAG}/README.en.md) · [日本語](https://github.com/{REPOSITORY}/blob/{TAG}/README.ja.md)

All assets passed independent offline consumer checks, ORT 1.26/1.29 CPU compatibility, original-API output comparisons and archive audits. Both Android ABIs ran actual OCR and host-runtime coexistence checks on these exact AARs.

Verify downloads with `SHA256SUMS` and `release-manifest.json`. Code: AGPL-3.0-only. Review `Model-NOTICE.txt` for the separately provided model and bundled third-party notices.
""", encoding="utf-8")
        existing = subprocess.run(["gh", "release", "view", TAG, "--repo", REPOSITORY, "--json", "isDraft,tagName"], text=True, capture_output=True, check=False)
        if existing.returncode == 0:
            require(json.loads(existing.stdout)["isDraft"] is True, "release is already published")
        else:
            gh("release", "create", TAG, "--repo", REPOSITORY, "--verify-tag", "--draft", "--prerelease", "--title", f"OneOCR {VERSION}", "--notes-file", body)
        names = sorted([*expected_assets(), "release-manifest.json", "SHA256SUMS"])
        gh("release", "upload", TAG, "--repo", REPOSITORY, *[assets / n for n in names], "--clobber")
        remote = json.loads(gh("api", f"repos/{REPOSITORY}/releases/tags/{TAG}"))
        require({a["name"] for a in remote["assets"]} == set(names), "uploaded asset set differs")
        for asset in remote["assets"]:
            require(asset.get("digest") == "sha256:" + digest(assets / asset["name"]) and asset["size"] == (assets / asset["name"]).stat().st_size, "uploaded checksum differs")
        gh("release", "edit", TAG, "--repo", REPOSITORY, "--draft=false", "--prerelease", "--notes-file", body)
        print(gh("release", "view", TAG, "--repo", REPOSITORY, "--json", "url,isDraft"))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    gather = commands.add_parser("collect")
    gather.add_argument("--run-id", required=True)
    gather.add_argument("--output", type=Path, required=True)
    gather.add_argument("--reports", type=Path, required=True)
    commands.add_parser("publish")
    args = parser.parse_args()
    if args.command == "collect":
        commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
        collect(args.run_id, commit, args.output, args.reports)
    else:
        kind = subprocess.check_output(["git", "cat-file", "-t", TAG], cwd=ROOT, text=True).strip()
        require(kind == "tag", "an annotated release receipt tag is required")
        message = subprocess.check_output(["git", "for-each-ref", "--format=%(contents)", "refs/tags/" + TAG], cwd=ROOT, text=True)
        publish(json.loads(message))

if __name__ == "__main__":
    main()
