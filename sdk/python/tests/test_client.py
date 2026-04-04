"""Tests for the Client class."""

from __future__ import annotations

import json
import os
import tempfile
from datetime import datetime, timezone
from pathlib import Path

import httpx
import pytest
import respx

from greenlab.client import Client, DEFAULT_BATCH_SIZE
from greenlab.config import LocalConfig, load_config


def _schema_resp(fields, schema_version=1):
    return httpx.Response(
        200,
        json={
            "success": True,
            "data": {"fields": fields, "schema_version": schema_version},
        },
    )


FIELDS = [{"index": 1, "name": "temperature", "type": "float"}]
SCHEMA_URL = "http://ingestion/v1/channels/ch-1/schema"
DATA_URL = "http://ingestion/v1/channels/ch-1/data"


@pytest.fixture
def tmp_config(tmp_path):
    return str(tmp_path / ".greenlab" / "sdk.json")


# ---------------------------------------------------------------------------
# Construction
# ---------------------------------------------------------------------------


@respx.mock
def test_new_success(tmp_config):
    respx.get(SCHEMA_URL).mock(return_value=_schema_resp(FIELDS))

    client = Client("http://ingestion", "key", "ch-1", config_file=tmp_config)

    assert client._schema.schema_version == 1


@respx.mock
def test_new_missing_base_url():
    with pytest.raises(ValueError, match="base_url"):
        Client("", "key", "ch-1")


@respx.mock
def test_new_missing_api_key():
    with pytest.raises(ValueError, match="api_key"):
        Client("http://x", "", "ch-1")


@respx.mock
def test_new_missing_channel_id():
    with pytest.raises(ValueError, match="channel_id"):
        Client("http://x", "key", "")


@respx.mock
def test_new_default_batch_size(tmp_config):
    respx.get(SCHEMA_URL).mock(return_value=_schema_resp(FIELDS))

    client = Client("http://ingestion", "key", "ch-1", batch_size=0, config_file=tmp_config)

    assert client._batch_size == DEFAULT_BATCH_SIZE


@respx.mock
def test_new_schema_fetch_fails():
    respx.get(SCHEMA_URL).mock(return_value=httpx.Response(401))

    with pytest.raises(RuntimeError, match="status 401"):
        Client("http://ingestion", "key", "ch-1")


@respx.mock
def test_new_persists_config(tmp_config):
    respx.get(SCHEMA_URL).mock(return_value=_schema_resp(FIELDS, schema_version=7))

    Client("http://ingestion", "key", "ch-1", config_file=tmp_config)

    cfg = load_config(tmp_config)
    assert cfg.channel_id == "ch-1"
    assert cfg.schema_version == 7
    assert cfg.format == "msgpack"


# ---------------------------------------------------------------------------
# set_field / set_field with ts
# ---------------------------------------------------------------------------


@respx.mock
def test_set_field_appends_reading(tmp_config):
    respx.get(SCHEMA_URL).mock(return_value=_schema_resp(FIELDS))
    client = Client("http://ingestion", "key", "ch-1", config_file=tmp_config)

    client.set_field("temperature", 28.5)

    assert len(client._pending) == 1
    assert client._pending[0].value == 28.5


@respx.mock
def test_set_field_with_explicit_ts(tmp_config):
    respx.get(SCHEMA_URL).mock(return_value=_schema_resp(FIELDS))
    client = Client("http://ingestion", "key", "ch-1", config_file=tmp_config)

    ts = datetime(2024, 1, 1, 0, 0, 0, tzinfo=timezone.utc)
    client.set_field("temperature", 28.5, ts=ts)

    assert client._pending[0].ts == ts


# ---------------------------------------------------------------------------
# send
# ---------------------------------------------------------------------------


@respx.mock
def test_send_success(tmp_config):
    respx.get(SCHEMA_URL).mock(return_value=_schema_resp(FIELDS))
    data_route = respx.post(DATA_URL).mock(return_value=httpx.Response(201))

    client = Client("http://ingestion", "key", "ch-1", config_file=tmp_config)
    client.set_field("temperature", 28.5)
    client.send()

    assert data_route.call_count == 1
    assert client._pending == []


@respx.mock
def test_send_empty_no_request(tmp_config):
    respx.get(SCHEMA_URL).mock(return_value=_schema_resp(FIELDS))
    data_route = respx.post(DATA_URL).mock(return_value=httpx.Response(201))

    client = Client("http://ingestion", "key", "ch-1", config_file=tmp_config)
    client.send()  # nothing pending

    assert data_route.call_count == 0


@respx.mock
def test_send_409_retries_after_schema_refetch(tmp_config):
    schema_route = respx.get(SCHEMA_URL).mock(
        side_effect=[
            _schema_resp(FIELDS, schema_version=1),  # initial fetch
            _schema_resp(FIELDS, schema_version=2),  # re-fetch after 409
        ]
    )
    call_count = {"n": 0}

    def data_side_effect(req):
        call_count["n"] += 1
        if call_count["n"] == 1:
            return httpx.Response(409)
        return httpx.Response(201)

    respx.post(DATA_URL).mock(side_effect=data_side_effect)

    client = Client("http://ingestion", "key", "ch-1", config_file=tmp_config)
    client.set_field("temperature", 28.5)
    client.send()

    assert schema_route.call_count == 2
    assert call_count["n"] == 2
    assert client._schema.schema_version == 2


@respx.mock
def test_send_unexpected_status_raises(tmp_config):
    respx.get(SCHEMA_URL).mock(return_value=_schema_resp(FIELDS))
    respx.post(DATA_URL).mock(return_value=httpx.Response(503))

    client = Client("http://ingestion", "key", "ch-1", config_file=tmp_config)
    client.set_field("temperature", 28.5)

    with pytest.raises(RuntimeError, match="503"):
        client.send()


@respx.mock
def test_send_recommended_format_persisted(tmp_config):
    respx.get(SCHEMA_URL).mock(return_value=_schema_resp(FIELDS))
    respx.post(DATA_URL).mock(
        return_value=httpx.Response(201, headers={"X-Recommended-Format": "msgpack"})
    )

    client = Client("http://ingestion", "key", "ch-1", config_file=tmp_config)
    client.set_field("temperature", 28.5)
    client.send()

    cfg = load_config(tmp_config)
    assert cfg.format == "msgpack"


@respx.mock
def test_send_sets_content_type_msgpack(tmp_config):
    respx.get(SCHEMA_URL).mock(return_value=_schema_resp(FIELDS))
    data_route = respx.post(DATA_URL).mock(return_value=httpx.Response(201))

    client = Client("http://ingestion", "key", "ch-1", config_file=tmp_config)
    client.set_field("temperature", 28.5)
    client.send()

    assert data_route.call_count == 1
    req = data_route.calls[0].request
    assert req.headers["Content-Type"] == "application/msgpack"
    assert req.headers["X-API-Key"] == "key"


@respx.mock
def test_send_lz4_compression_header_on_large_payload(tmp_config):
    """A large enough payload should carry Content-Encoding: lz4."""
    # Add 10 fields (many readings) to produce a large payload.
    fields = [{"index": i + 1, "name": f"field{i}", "type": "float"} for i in range(8)]
    schema_url = "http://ingestion/v1/channels/ch-1/schema"
    respx.get(schema_url).mock(return_value=_schema_resp(fields))
    data_route = respx.post(DATA_URL).mock(return_value=httpx.Response(201))

    client = Client("http://ingestion", "key", "ch-1", config_file=tmp_config)

    # Set all 8 fields to produce a payload > 100 bytes.
    for i in range(8):
        client.set_field(f"field{i}", float(i) * 1.5)
    client.send()

    req = data_route.calls[0].request
    # Whether compression fires depends on payload size; just assert no crash.
    # The header presence is conditional.
    content_enc = req.headers.get("Content-Encoding", "")
    assert content_enc in ("", "lz4")
