class OneOcrError(Exception):
    """An actionable input, format, cache, or inference error."""


class ModelFormatError(OneOcrError):
    """The model does not satisfy the supported container/profile invariants."""


class UnsupportedModelError(OneOcrError):
    """A valid container requires an unimplemented OCR architecture."""
