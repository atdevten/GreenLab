"""Batch building and td-offset calculation for the GreenLab SDK."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from typing import List, Optional

import msgpack

from .schema import ChannelSchema

MAX_TD_MILLIS = 65535


@dataclass
class Reading:
    field_name: str
    value: float
    ts: datetime


def build_batches(
    readings: List[Reading],
    schema: ChannelSchema,
    schema_version: int,
) -> List[bytes]:
    """Partition *readings* into one or more MessagePack-encoded batch payloads.

    Readings within a batch share a base timestamp (the timestamp of the first
    reading in the batch). Per-field ``td`` values are millisecond offsets from
    the base. If any delta exceeds ``MAX_TD_MILLIS`` (65535ms), the current
    batch is flushed and a new one begins at the offending reading's timestamp.

    Returns a list of raw MessagePack bytes, one element per batch.
    Raises :class:`ValueError` for empty input or unknown field names.
    """
    if not readings:
        raise ValueError("sdk: no readings to send")

    num_fields = len(schema.ordered_fields)
    if num_fields == 0:
        raise ValueError("sdk: schema has no fields")

    # Build fieldName → 0-based position in the f array.
    field_pos = {fe.name: i for i, fe in enumerate(schema.ordered_fields)}

    for r in readings:
        if r.field_name not in field_pos:
            raise ValueError(f"sdk: unknown field {r.field_name!r}")

    result: List[bytes] = []
    base_ts: Optional[datetime] = None
    f_vals: List[Optional[float]] = [None] * num_fields
    td_vals: List[Optional[int]] = [None] * num_fields
    batch_used = False

    def flush() -> None:
        nonlocal batch_used
        # Only include slots with values.
        f: List[float] = []
        td: List[int] = []
        for i in range(num_fields):
            if f_vals[i] is not None:
                f.append(f_vals[i])  # type: ignore[arg-type]
                td.append(td_vals[i])  # type: ignore[arg-type]
        if not f:
            return
        assert base_ts is not None
        payload = {
            "sv": schema_version,
            "ts": int(base_ts.timestamp()),
            "f": f,
            "td": td,
        }
        result.append(msgpack.packb(payload, use_bin_type=True))
        batch_used = False

    def reset(new_base: datetime) -> None:
        nonlocal base_ts, f_vals, td_vals, batch_used
        base_ts = new_base
        f_vals = [None] * num_fields
        td_vals = [None] * num_fields
        batch_used = False

    reset(readings[0].ts)

    for r in readings:
        assert base_ts is not None
        delta_ms = int((r.ts - base_ts).total_seconds() * 1000)
        if delta_ms < 0:
            delta_ms = 0

        if delta_ms > MAX_TD_MILLIS:
            if batch_used:
                flush()
            reset(r.ts)
            delta_ms = 0

        pos = field_pos[r.field_name]
        f_vals[pos] = r.value
        td_vals[pos] = delta_ms
        batch_used = True

    if batch_used:
        flush()

    return result
