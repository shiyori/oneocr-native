"""Protect the complete-only Linux download contract."""
import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from audit_release import audit_zip
from build_sdk import archive
from release_manifest import expected_assets
from runtime_assets import digest
from version import VERSION


class LinuxLayoutTests(unittest.TestCase):
    def test_release_only_has_one_complete_package_per_linux_architecture(self):
        expected = expected_assets()
        linux = {name: kind for name, (kind, platform) in expected.items() if platform.startswith("linux-")}
        self.assertEqual(linux, {f"oneocr-linux-{arch}.zip": "app-full" for arch in ("amd64", "arm64")})
        self.assertFalse(any(name.startswith("oneocr-runtime-") for name in expected))

    def package(self, root, *, extra=None, missing=None, platform="linux-amd64"):
        stage = root / "oneocr-linux-amd64"
        files = {"bin/oneocr", "lib/libonnxruntime.so", "models/oneocr-cjk-en.ocrpack", "licenses/LICENSE.txt"}
        if extra:
            files.add(extra)
        if missing:
            files.remove(missing)
        for name in files:
            path = stage / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b"test payload")
        records = [{"file": name, "bytes": (stage / name).stat().st_size, "sha256": digest(stage / name)} for name in sorted(files)]
        (stage / "PACKAGE.json").write_text(json.dumps({"schema": "oneocr.distribution.v1", "version": VERSION,
            "platform": platform, "entrypoint": "bin/oneocr", "runtime_included": True, "models_included": True, "files": records}))
        output = root / "oneocr-linux-amd64.zip"
        archive(stage, output)
        return output

    def test_accepts_complete_runtime_layout(self):
        with tempfile.TemporaryDirectory() as temporary:
            audit_zip(self.package(Path(temporary)), "app-full")

    def test_rejects_sdk_headers(self):
        with tempfile.TemporaryDirectory() as temporary, self.assertRaisesRegex(RuntimeError, "development-only|SDK payload"):
            audit_zip(self.package(Path(temporary), extra="include/oneocr.h"), "app-full")

    def test_rejects_missing_runtime(self):
        with tempfile.TemporaryDirectory() as temporary, self.assertRaisesRegex(RuntimeError, "omits runtime"):
            audit_zip(self.package(Path(temporary), missing="lib/libonnxruntime.so"), "app-full")

    def test_rejects_wrong_architecture_record(self):
        with tempfile.TemporaryDirectory() as temporary, self.assertRaisesRegex(RuntimeError, "incomplete Linux package record"):
            audit_zip(self.package(Path(temporary), platform="linux-arm64"), "app-full")


if __name__ == "__main__":
    unittest.main()
