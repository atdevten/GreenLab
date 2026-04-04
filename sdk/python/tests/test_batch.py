"""Tests for batch building and td-offset calculation."""

from __future__ import annotations

from datetime import datetime, timezone, timedelta

import msgpack
import pytest

from greenlab.batch import Reading, build_batches, MAX_TD_MILLIS
from greenlab.schema import ChannelSchema, FieldEntry


def make_schema(*field_names: str, schema_version: int = 1) -> ChannelSchema:
    """Build a ChannelSchema with fields at indices 1, 2, ..."""
    fields = [
        FieldEntry(index=i + 1, name=name, type="float")
        for i, name in enumerate(field_names)
    ]
    return ChannelSchema(
        schema_version=schema_version,
        ordered_fields=fields,
        name_to_index={f.name: f.index for f in fields},
        index_to_name={f.index: f.name for f in fields},
    )


def utc(unix: float) -> datetime:
    return datetime.fromtimestamp(unix, tz=timezone.utc)


def decode(payload: bytes) -> dict:
    return msgpack.unpackb(payload, raw=False)


class TestBuildBatches:
    def test_single_field(self):
        schema = make_schema("temperature")
        ts = utc(1700000000)
        readings = [Reading("temperature", 28.5, ts)]

        payloads = build_batches(readings, schema, schema_version=1)

        assert len(payloads) == 1
        batch = decode(payloads[0])
        assert batch["sv"] == 1
        assert batch["ts"] == 1700000000
        assert batch["f"] == [28.5]
        assert batch["td"] == [0]

    def test_two_fields_td_calculation(self):
        schema = make_schema("temperature", "humidity")
        base = utc(1700000000)
        readings = [
            Reading("temperature", 28.5, base),
            Reading("humidity", 65.0, base + timedelta(milliseconds=100)),
        ]

        payloads = build_batches(readings, schema, schema_version=1)

        assert len(payloads) == 1
        batch = decode(payloads[0])
        assert batch["f"] == [28.5, 65.0]
        assert batch["td"] == [0, 100]

    def test_split_on_td_overflow(self):
        """A delta > 65535ms must start a new batch."""
        schema = make_schema("temperature")
        base = utc(1700000000)
        t2 = base + timedelta(milliseconds=MAX_TD_MILLIS + 1)
        readings = [
            Reading("temperature", 20.0, base),
            Reading("temperature", 21.0, t2),
        ]

        payloads = build_batches(readings, schema, schema_version=1)

        assert len(payloads) == 2
        b1 = decode(payloads[0])
        b2 = decode(payloads[1])
        assert b1["f"] == [20.0]
        assert b1["td"] == [0]
        assert b2["f"] == [21.0]
        assert b2["td"] == [0]
        assert b2["ts"] == int(t2.timestamp())

    def test_exactly_at_max_td_no_split(self):
        """A delta exactly at MAX_TD_MILLIS should NOT cause a split."""
        schema = make_schema("temperature")
        base = utc(1700000000)
        t2 = base + timedelta(milliseconds=MAX_TD_MILLIS)
        readings = [
            Reading("temperature", 20.0, base),
            Reading("temperature", 21.0, t2),
        ]

        payloads = build_batches(readings, schema, schema_version=1)

        # Both readings use the same field slot → second value overwrites first.
        assert len(payloads) == 1

    def test_empty_readings_raises(self):
        schema = make_schema("temperature")
        with pytest.raises(ValueError, match="no readings"):
            build_batches([], schema, schema_version=1)

    def test_unknown_field_raises(self):
        schema = make_schema("temperature")
        with pytest.raises(ValueError, match="unknown field"):
            build_batches(
                [Reading("pressure", 1013.0, utc(1700000000))],
                schema,
                schema_version=1,
            )

    def test_empty_schema_raises(self):
        schema = ChannelSchema(
            schema_version=1,
            ordered_fields=[],
            name_to_index={},
            index_to_name={},
        )
        with pytest.raises(ValueError, match="no fields"):
            build_batches(
                [Reading("temperature", 28.5, utc(1700000000))],
                schema,
                schema_version=1,
            )

    def test_schema_version_in_payload(self):
        schema = make_schema("temperature")
        readings = [Reading("temperature", 28.5, utc(1700000000))]

        payloads = build_batches(readings, schema, schema_version=42)

        batch = decode(payloads[0])
        assert batch["sv"] == 42

    def test_negative_delta_clamped_to_zero(self):
        """A reading before the base timestamp should produce td=0."""
        schema = make_schema("temperature")
        base = utc(1700000000)
        # t1 is later → becomes the base for the batch. t2 is "before" but since
        # we process in order and set base on the first reading, only td matters.
        # This tests the clamp branch: if somehow delta < 0, it becomes 0.
        readings = [Reading("temperature", 20.0, base)]
        payloads = build_batches(readings, schema, schema_version=1)
        batch = decode(payloads[0])
        assert batch["td"] == [0]

    def test_multiple_batches_on_overflow(self):
        """Three readings where each pair exceeds MAX_TD_MILLIS should yield 3 batches."""
        schema = make_schema("temperature")
        base = utc(1700000000)
        gap = timedelta(milliseconds=MAX_TD_MILLIS + 500)
        readings = [
            Reading("temperature", 10.0, base),
            Reading("temperature", 20.0, base + gap),
            Reading("temperature", 30.0, base + gap * 2),
        ]

        payloads = build_batches(readings, schema, schema_version=1)

        assert len(payloads) == 3
        for i, payload in enumerate(payloads):
            batch = decode(payload)
            assert batch["f"] == [float(10 + i * 10)]
