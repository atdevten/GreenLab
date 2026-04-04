"""LZ4 compression helpers for the GreenLab SDK."""

from __future__ import annotations

from typing import Tuple

import lz4.frame

COMPRESSION_THRESHOLD = 100


def maybe_compress(payload: bytes) -> Tuple[bytes, bool]:
    """LZ4-compress *payload* if it exceeds ``COMPRESSION_THRESHOLD`` bytes.

    Returns a ``(data, did_compress)`` tuple. If compression is not applied,
    *data* is the original *payload* unchanged.
    """
    if len(payload) <= COMPRESSION_THRESHOLD:
        return payload, False
    compressed = lz4.frame.compress(payload)
    return compressed, True
