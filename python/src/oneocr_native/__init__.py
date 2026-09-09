"""OneOCR's Python SDK. Installation helpers do not import inference dependencies."""
from ._version import __version__
from .errors import ModelFormatError, OneOcrError, UnsupportedModelError

__all__ = [
    "DetectionRegion", "DetectionResult", "EngineConfig", "LineResult",
    "ModelFormatError", "OcrLine", "OcrResult", "OneOcrEngine", "OneOcrError",
    "UnsupportedModelError", "__version__",
]


def __getattr__(name: str):
    if name in {"DetectionRegion", "DetectionResult", "EngineConfig", "LineResult", "OcrLine", "OcrResult", "OneOcrEngine"}:
        from . import engine
        value = getattr(engine, name)
        globals()[name] = value
        return value
    raise AttributeError(name)
