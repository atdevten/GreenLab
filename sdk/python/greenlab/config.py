"""Local config persistence for the GreenLab SDK."""

from __future__ import annotations

import json
import os
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Optional

DEFAULT_CONFIG_FILE = "~/.greenlab/sdk.json"


@dataclass
class LocalConfig:
    channel_id: str = ""
    schema_version: int = 0
    format: str = "msgpack"


def _expand(path: str) -> Path:
    return Path(path).expanduser()


def load_config(path: str) -> LocalConfig:
    """Load the local config from *path*.

    Returns a default :class:`LocalConfig` if the file does not exist.
    """
    p = _expand(path)
    if not p.exists():
        return LocalConfig()
    data = json.loads(p.read_text())
    return LocalConfig(
        channel_id=data.get("channel_id", ""),
        schema_version=data.get("schema_version", 0),
        format=data.get("format", "msgpack"),
    )


def save_config(path: str, cfg: LocalConfig) -> None:
    """Persist *cfg* to *path*, creating parent directories as needed."""
    p = _expand(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(json.dumps(asdict(cfg), indent=2))
