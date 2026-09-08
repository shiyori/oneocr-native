import struct
from hashlib import sha256

import pytest
from Crypto.Cipher import AES
from Crypto.Util.Padding import pad

from oneocr_native.container import IV, MARKER, MASTER_KEY, ModelContainer, decrypt_record
from oneocr_native.errors import ModelFormatError


def u64(value):
    return struct.pack("<Q", value)


def blob(value):
    return u64(len(value)) + value


def record(payload, *, password=b"", salted=True, marker=MARKER):
    # Independent fixture writer: clear key-length header or password envelope.
    salt = bytes(range(16)) if salted else b""
    lengths = u64(len(payload)) + u64(len(payload) + 24)
    key = sha256((password or lengths) + salt).digest()
    plain = (lengths if password else b"") + u64(marker) + payload
    encrypted = AES.new(key, AES.MODE_CBC, IV).encrypt(pad(plain, 16))
    return salt + (b"" if password else lengths) + encrypted


def container_bytes(resources=None, *, config=b"config", offset=0, flag=1, name=None):
    resources = resources or [(r"C:\models\a.bin", b"first"), (r"C:\models\b.bin", b"second")]
    index = blob(record(config)) + u64(len(resources))
    body = b""
    for i, (source_name, payload) in enumerate(resources):
        encoded = record(payload)
        index += (
            blob(record((name or source_name).encode(), salted=False))
            + u64(len(body) + (offset if i == 0 else 0))
            + u64(len(encoded))
            + bytes([flag])
        )
        body += encoded
    return blob(record(index, password=MASTER_KEY)) + blob(body)


@pytest.mark.parametrize("length", [0, 1, 8, 15, 16, 17, 31, 64, 4097])
@pytest.mark.parametrize("password,salted", [(MASTER_KEY, True), (b"", True), (b"", False)])
def test_envelope_lengths_and_padding(length, password, salted):
    payload = bytes(i % 251 for i in range(length))
    assert (
        decrypt_record(
            record(payload, password=password, salted=salted), password=password, salted=salted
        )
        == payload
    )


def test_complete_resource_index():
    data = container_bytes()
    model = ModelContainer.from_bytes(data)
    assert model.source_hash == sha256(data).hexdigest()
    assert model.config == b"config"
    assert [resource.data for resource in model.resources] == [b"first", b"second"]
    assert [resource.filename for resource in model.resources] == ["0000.bin", "0001.bin"]


@pytest.mark.parametrize("cut", [0, 1, 7, 8, 24, 64, -1, -16])
def test_rejects_truncation(cut):
    with pytest.raises(ModelFormatError):
        ModelContainer.from_bytes(container_bytes()[:cut])


@pytest.mark.parametrize("kwargs", [{"offset": 16}, {"flag": 0}, {"name": "duplicate"}])
def test_rejects_invalid_index(kwargs):
    with pytest.raises(ModelFormatError):
        ModelContainer.from_bytes(container_bytes(**kwargs))


def test_unknown_crypto_version():
    with pytest.raises(ModelFormatError, match="marker"):
        decrypt_record(record(b"payload", marker=MARKER + 1))


def test_corrupt_payload_rejected():
    broken = bytearray(container_bytes())
    broken[-1] ^= 1
    with pytest.raises(ModelFormatError):
        ModelContainer.from_bytes(bytes(broken))


def test_no_trailing_data():
    with pytest.raises(ModelFormatError, match="trailing"):
        ModelContainer.from_bytes(container_bytes() + b"unexpected")


def test_clear_length_tampering():
    broken = bytearray(record(b"payload"))
    broken[16] ^= 1
    with pytest.raises(ModelFormatError):
        decrypt_record(bytes(broken))
