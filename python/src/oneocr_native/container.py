"""Reader for the CBC OneModel container, without executing or loading any DLL.

Recovered from the supplied x64 DLL: Crypto at RVA 0x614730 (CBC/SHA256),
envelope at 0x6155c0, file reader at 0x613880, index parser at 0x613c70.
The envelope's lengths and marker are structural checks, NOT authentication.
"""

from __future__ import annotations

import struct
from dataclasses import dataclass
from hashlib import sha256
from pathlib import Path

from Crypto.Cipher import AES
from Crypto.Util.Padding import unpad

from .errors import ModelFormatError

MASTER_KEY = b"kj)TGtrK>f]b[Piow.gU+nC@s" + b'"' * 6 + b"4"
IV = b"Copyright @ OneO"
MARKER = 0x252B081A4A
MAX_FILE_BYTES = 2 * 1024**3
MAX_ENTRIES = 4096


class Reader:
    def __init__(self, data: bytes, label: str):
        self.data = data
        self.pos = 0
        self.label = label

    def take(self, size: int) -> bytes:
        if size < 0 or size > len(self.data) - self.pos:
            raise ModelFormatError(
                f"{self.label}: truncated field at byte {self.pos} ({size} bytes)"
            )
        result = self.data[self.pos : self.pos + size]
        self.pos += size
        return result

    def u64(self) -> int:
        return struct.unpack("<Q", self.take(8))[0]

    def blob(self, limit: int = MAX_FILE_BYTES) -> bytes:
        size = self.u64()
        if size > limit:
            raise ModelFormatError(f"{self.label}: field exceeds limit ({size} > {limit})")
        return self.take(size)

    def finish(self) -> None:
        if self.pos != len(self.data):
            raise ModelFormatError(f"{self.label}: unexpected trailing bytes")


def decrypt_record(data: bytes, *, password: bytes = b"", salted: bool = True) -> bytes:
    """Decode one record and validate PKCS#7, clear/encrypted lengths and marker."""
    reader = Reader(data, "encrypted record")
    salt = reader.take(16) if salted else b""
    explicit_password = bool(password)
    clear_lengths = b"" if explicit_password else reader.take(16)
    key_material = password if explicit_password else clear_lengths
    ciphertext = reader.take(len(data) - reader.pos)
    if not ciphertext or len(ciphertext) % AES.block_size:
        raise ModelFormatError("encrypted record: invalid AES-CBC block length")
    key = sha256(key_material + salt).digest()
    try:
        plain = unpad(AES.new(key, AES.MODE_CBC, IV).decrypt(ciphertext), AES.block_size)
    except ValueError as exc:
        raise ModelFormatError(
            "encrypted record: invalid padding; wrong key/version or damage"
        ) from exc
    envelope = Reader(clear_lengths + plain, "decrypted envelope")
    payload_size, total_size, marker = envelope.u64(), envelope.u64(), envelope.u64()
    if marker != MARKER or total_size != payload_size + 24:
        raise ModelFormatError("encrypted record: unsupported marker or inconsistent lengths")
    payload = envelope.take(payload_size)
    envelope.finish()
    return payload


@dataclass(frozen=True)
class Resource:
    index: int
    name: str
    offset: int
    stored_size: int
    data: bytes

    @property
    def filename(self) -> str:
        return f"{self.index:04d}" + (".onnx" if self.name.lower().endswith(".onnx") else ".bin")

    @property
    def digest(self) -> str:
        return sha256(self.data).hexdigest()


@dataclass(frozen=True)
class ModelContainer:
    source_hash: str
    source_size: int
    config: bytes
    resources: tuple[Resource, ...]

    @classmethod
    def load(cls, path: str | Path) -> ModelContainer:
        path = Path(path)
        if path.stat().st_size > MAX_FILE_BYTES:
            raise ModelFormatError("model exceeds the supported 2 GiB size limit")
        return cls.from_bytes(path.read_bytes())

    @classmethod
    def from_bytes(cls, data: bytes) -> ModelContainer:
        if len(data) > MAX_FILE_BYTES:
            raise ModelFormatError("model exceeds the supported 2 GiB size limit")
        file = Reader(data, "OneModel file")
        index_data = decrypt_record(file.blob(), password=MASTER_KEY)
        body = file.blob()
        file.finish()
        index = Reader(index_data, "resource index")
        config = decrypt_record(index.blob())
        count = index.u64()
        if not 0 < count <= MAX_ENTRIES:
            raise ModelFormatError(f"resource index: invalid entry count {count}")
        resources: list[Resource] = []
        names: set[str] = set()
        next_offset = 0
        for i in range(count):
            try:
                name = decrypt_record(index.blob(16384), salted=False).decode("utf-8")
            except UnicodeDecodeError as exc:
                raise ModelFormatError("resource index: invalid UTF-8 name") from exc
            if not name or "\x00" in name or name in names:
                raise ModelFormatError("resource index: empty, invalid, or duplicate name")
            names.add(name)
            offset, size = index.u64(), index.u64()
            flag = index.take(1)[0]
            if flag != 1:
                raise ModelFormatError(f"resource {i}: unsupported encoding flag {flag}")
            if offset != next_offset or size > len(body) - offset:
                raise ModelFormatError(
                    f"resource {i}: overlapping, noncontiguous or out-of-bounds data"
                )
            payload = decrypt_record(body[offset : offset + size])
            resources.append(Resource(i, name, offset, size, payload))
            next_offset = offset + size
        index.finish()
        if next_offset != len(body):
            raise ModelFormatError("resource index: does not cover the complete data section")
        return cls(sha256(data).hexdigest(), len(data), config, tuple(resources))
