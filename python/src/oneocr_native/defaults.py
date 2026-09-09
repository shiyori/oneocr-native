from __future__ import annotations

import hashlib
import json
import os
import sys
from pathlib import Path

from .errors import ModelFormatError

DEFAULT_MODEL = "oneocr-cjk-en.ocrpack"


def default_model_path(home: Path | None = None) -> Path:
    """Find the single default model without selecting other .ocrpack files."""
    if explicit := os.environ.get("ONEOCR_MODEL"):
        return Path(explicit)
    executable = Path(sys.executable).resolve().parent
    for root in (Path.cwd(), executable, executable.parent):
        for candidate in (root / DEFAULT_MODEL, root / "models" / DEFAULT_MODEL):
            if candidate.is_file():
                return candidate.resolve()
    home = home or installation_home()
    config = home / "config.json"
    if config.is_file():
        if config.stat().st_size > 1024 * 1024:
            raise ModelFormatError("installed model configuration is too large")
        installed = json.loads(config.read_text(encoding="utf-8"))
        if (
            not isinstance(installed, dict)
            or installed.get("schema") not in ("oneocr.install.v1", "oneocr.install.v2")
            or installed.get("profile") != "cjk-en"
            or not isinstance(installed.get("model_path"), str)
            or not installed["model_path"]
        ):
            raise ModelFormatError(f"install the default {DEFAULT_MODEL} model")
        model = Path(installed["model_path"])
        with model.open("rb") as stream:
            actual = hashlib.file_digest(stream, "sha256").hexdigest()
        if actual != installed.get("package_sha256"):
            raise ModelFormatError("installed model package checksum mismatch")
        return model
    raise FileNotFoundError(
        f"default model {DEFAULT_MODEL} not found; place it in models/ or run python -m oneocr_native install"
    )


def installation_home() -> Path:
    if configured := os.environ.get("ONEOCR_HOME"):
        home = Path(configured)
    elif sys.platform == "darwin":
        home = Path.home() / "Library" / "Application Support" / "oneocr"
    elif sys.platform == "win32":
        home = Path(os.environ.get("APPDATA", Path.home() / "AppData" / "Roaming")) / "oneocr"
    else:
        home = Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config")) / "oneocr"
    return home
