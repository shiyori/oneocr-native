"""Visual-to-logical RTL ordering using macOS's native ICU implementation.

OCR recovers visual order; the original logical order is not uniquely
recoverable for every mixed-direction string. ICU's inverse-like-direct mode
preserves numeric runs and combining marks instead of reversing code points.
See https://unicode-org.github.io/icu-docs/apidoc/released/icu4c/ubidi_8h.html
"""

import ctypes as c
import sys
from functools import lru_cache

from .errors import UnsupportedModelError


@lru_cache(maxsize=1)
def _icu():
    if sys.platform != "darwin":
        raise UnsupportedModelError("RTL reordering currently requires macOS system ICU")
    try:
        lib = c.CDLL("/usr/lib/libicucore.A.dylib")
        lib.ubidi_open.argtypes = []
        lib.ubidi_open.restype = c.c_void_p
        lib.ubidi_close.argtypes = [c.c_void_p]
        lib.ubidi_close.restype = None
        lib.ubidi_setReorderingMode.argtypes = [c.c_void_p, c.c_int]
        lib.ubidi_setReorderingMode.restype = None
        lib.ubidi_setPara.argtypes = [
            c.c_void_p,
            c.POINTER(c.c_uint16),
            c.c_int32,
            c.c_uint8,
            c.POINTER(c.c_uint8),
            c.POINTER(c.c_int32),
        ]
        lib.ubidi_setPara.restype = None
        lib.ubidi_writeReordered.argtypes = [
            c.c_void_p,
            c.POINTER(c.c_uint16),
            c.c_int32,
            c.c_uint16,
            c.POINTER(c.c_int32),
        ]
        lib.ubidi_writeReordered.restype = c.c_int32
        return lib
    except (OSError, AttributeError) as exc:
        raise UnsupportedModelError("macOS ICU bidi functions are unavailable") from exc


def visual_to_logical(text: str) -> str:
    if not text:
        return text
    if sys.platform != "darwin":
        return portable_visual_to_logical(text)
    lib = _icu()
    encoded = text.encode("utf-16-le")
    length = len(encoded) // 2
    source = (c.c_uint16 * length).from_buffer_copy(encoded)
    destination = (c.c_uint16 * (length * 2 + 16))()
    status = c.c_int32(0)
    bidi = lib.ubidi_open()
    if not bidi:
        raise UnsupportedModelError("ICU could not allocate a bidi context")
    try:
        # UBIDI_REORDER_INVERSE_LIKE_DIRECT = 5; paragraph level 1 = RTL.
        lib.ubidi_setReorderingMode(bidi, 5)
        lib.ubidi_setPara(bidi, source, length, 1, None, c.byref(status))
        if status.value > 0:
            raise UnsupportedModelError(f"ICU bidi paragraph error {status.value}")
        # KEEP_BASE_COMBINING | DO_MIRRORING. No synthetic directional marks.
        written = lib.ubidi_writeReordered(bidi, destination, len(destination), 3, c.byref(status))
        if status.value > 0 or not 0 <= written <= len(destination):
            raise UnsupportedModelError(f"ICU bidi output error {status.value}")
        return bytes(destination)[: written * 2].decode("utf-16-le")
    finally:
        lib.ubidi_close(bidi)


def portable_visual_to_logical(text: str) -> str:
    """Inverse-bidi approximation matching Go, with combining and numeric runs.

    Common paired punctuation is mirrored. Nested isolates, directional controls
    and arbitrary mixed-direction text are ambiguous and not fully reconstructed.
    """
    import unicodedata as ud

    clusters = []
    for char in text:
        if ud.category(char) in ("Mn", "Mc", "Me") and clusters:
            clusters[-1][0] += char
        else:
            kind = {"L": 1, "EN": 2, "AN": 2, "R": 3, "AL": 3}.get(ud.bidirectional(char), 0)
            clusters.append([char, kind])
    i = 0
    while i < len(clusters):
        if clusters[i][1]:
            i += 1
            continue
        end = i
        while end < len(clusters) and not clusters[end][1]:
            end += 1
        if i and end < len(clusters) and clusters[i - 1][1] == clusters[end][1]:
            kind = clusters[i - 1][1]
            space = any(c.isspace() for value, _ in clusters[i:end] for c in value)
            if kind in (1, 3) or (kind == 2 and not space):
                for j in range(i, end):
                    clusters[j][1] = kind
        i = end
    runs = []
    for value, kind in clusters:
        if not runs or runs[-1][0] != kind:
            runs.append((kind, []))
        runs[-1][1].append(value)
    mirrors = str.maketrans("()[]{}<>«»‹›", ")(][}{><»«›‹")
    output = []
    for kind, values in reversed(runs):
        output.extend(
            values
            if kind in (1, 2)
            else [value[0].translate(mirrors) + value[1:] for value in reversed(values)]
        )
    return "".join(output)
