"""Keep publication from mixing commits, platforms or changed artifacts."""
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import publish_release as release
from runtime_assets import digest


class AssembleTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.packages = self.root / "packages"
        self.packages.mkdir()
        self.output = self.root / "output"
        self.expected = {"shared.txt": ("license", "")}
        for platform in release.BUILD_PLATFORMS:
            self.expected[platform + ".zip"] = ("runtime", platform)
            directory = self.packages / ("release-assets-" + platform)
            directory.mkdir()
            (directory / (platform + ".zip")).write_bytes(platform.encode())
            (directory / "shared.txt").write_bytes(b"shared")
            for name in ("SHA256SUMS", "release-manifest.json"):
                (directory / name).write_text("metadata")
            record = {"schema": "oneocr.build.v1", "version": release.VERSION,
                      "commit": "current", "platform": platform,
                      "assets": {name: digest(directory / name) for name in (platform + ".zip", "shared.txt")}}
            (directory / release.RECORD).write_text(json.dumps(record))
        for name, value in (("current_commit", "current"), ("expected_assets", self.expected),
                            ("create", None), ("audit", {"passed": True})):
            patcher = patch.object(release, name, return_value=value)
            self.addCleanup(patcher.stop)
            patcher.start()

    def change_record(self, update):
        path = self.packages / "release-assets-android" / release.RECORD
        record = json.loads(path.read_text())
        update(record)
        path.write_text(json.dumps(record))

    def test_collects_same_commit_and_deduplicates_shared_files(self):
        release.assemble(self.packages, self.output)
        self.assertEqual({p.name for p in self.output.iterdir()}, set(self.expected))

    def test_rejects_a_different_source_commit(self):
        self.change_record(lambda record: record.update(commit="old"))
        with self.assertRaisesRegex(RuntimeError, "tagged source"):
            release.assemble(self.packages, self.output)

    def test_rejects_changed_download(self):
        (self.packages / "release-assets-android/android.zip").write_bytes(b"damaged")
        with self.assertRaisesRegex(RuntimeError, "checksum mismatch"):
            release.assemble(self.packages, self.output)

    def test_rejects_conflicting_shared_asset(self):
        source = self.packages / "release-assets-android/shared.txt"
        source.write_bytes(b"different, but with a matching local record")
        self.change_record(lambda record: record["assets"].update({"shared.txt": digest(source)}))
        with self.assertRaisesRegex(RuntimeError, "shared asset differs"):
            release.assemble(self.packages, self.output)

    def test_rejects_unrecorded_files(self):
        (self.packages / "release-assets-android/unexpected.txt").write_text("extra")
        with self.assertRaisesRegex(RuntimeError, "omits artifact files"):
            release.assemble(self.packages, self.output)

    def test_rejects_missing_platform(self):
        (self.packages / "release-assets-android").rename(self.root / "absent")
        with self.assertRaisesRegex(RuntimeError, "missing or unexpected"):
            release.assemble(self.packages, self.output)


class RecordTests(unittest.TestCase):
    def test_ignores_uv_hidden_output_marker(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / ".gitignore").write_text("*")
            (directory / "model.ocrpack").write_bytes(b"model")
            for name in ("release-manifest.json", "SHA256SUMS"):
                (directory / name).write_text("metadata")
            with patch.object(release, "create"), patch.object(release, "audit"), \
                 patch.object(release, "current_commit", return_value="tagged"), \
                 patch.object(release, "expected_assets", return_value={"model.ocrpack": ("model", "")}):
                release.record(directory, "linux-amd64")
            record = json.loads((directory / release.RECORD).read_text())
            self.assertEqual(set(record["assets"]), {"model.ocrpack"})
            self.assertEqual(record["commit"], "tagged")


class UploadedTests(unittest.TestCase):
    def test_uses_draft_aware_asset_lookup(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            path = directory / "asset.zip"
            path.write_bytes(b"asset")
            remote = {"assets": [{"name": path.name, "size": path.stat().st_size, "digest": "sha256:" + digest(path)}]}
            with patch.object(release, "gh", return_value=json.dumps(remote)) as gh:
                release.verify_uploaded(directory, [path.name])
                gh.assert_called_once_with("release", "view", release.TAG, "--repo", release.REPOSITORY, "--json", "assets")
            remote["assets"][0]["digest"] = "sha256:wrong"
            with patch.object(release, "gh", return_value=json.dumps(remote)), \
                 self.assertRaisesRegex(RuntimeError, "uploaded checksum differs"):
                release.verify_uploaded(directory, [path.name])


if __name__ == "__main__":
    unittest.main()
