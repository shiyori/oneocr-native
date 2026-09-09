# Model package format

[简体中文](../zh-CN/model-format.md) | [English](../en/model-format.md) | [日本語](../ja/model-format.md)

The default `oneocr-cjk-en.ocrpack` is a single ONEOCRPK container. SDKs verify its metadata and file hashes before creating model sessions; no resource directory needs to be extracted during normal use.

The 64-byte header stores `ONEOCRPK` at bytes 0–7, a little-endian version and flags at 8–15, the JSON index length at 16–23, the data-section offset at 24–31, and the index SHA-256 at 32–63. JSON starts at byte 64. The data section and file ranges use 64-byte alignment; file offsets are relative to the data section. Metadata records the profile, source model hash, pipeline, file sizes and SHA-256 hashes. The reader rejects unsafe names, duplicates, overlapping ranges and hash mismatches.

The default package contains the detector, classifier, CJK/English recognizer and associated dictionaries. Recognition keeps the original CPU model graph and its existing result semantics.

Developer commands `oneocr pack`, `inspect` and `unpack` create or inspect packages. `oneocr export` retains legacy directory-bundle support. Use `Pack`, `ReadPackage` and `Unpack` for Go tooling. These are packaging tools, separate from the three image operations; applications can simply use the default installed model.

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)
