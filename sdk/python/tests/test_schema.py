"""Tests for schema fetching and caching."""

import pytest
import respx
import httpx

from greenlab.schema import fetch_schema, _build_schema, ChannelSchema, FieldEntry


SCHEMA_URL = "http://test-host/v1/channels/ch-1/schema"


def _schema_response(fields, schema_version=1):
    return {
        "success": True,
        "data": {
            "fields": fields,
            "schema_version": schema_version,
        },
    }


@respx.mock
def test_fetch_schema_success():
    respx.get(SCHEMA_URL).mock(
        return_value=httpx.Response(
            200,
            json=_schema_response(
                [
                    {"index": 1, "name": "temperature", "type": "float"},
                    {"index": 2, "name": "humidity", "type": "float"},
                ],
                schema_version=3,
            ),
        )
    )

    schema = fetch_schema("http://test-host", "ch-1", "my-key")

    assert schema.schema_version == 3
    assert schema.name_to_index["temperature"] == 1
    assert schema.name_to_index["humidity"] == 2
    assert schema.index_to_name[1] == "temperature"
    assert schema.index_to_name[2] == "humidity"
    assert len(schema.ordered_fields) == 2


@respx.mock
def test_fetch_schema_sends_api_key_header():
    route = respx.get(SCHEMA_URL).mock(
        return_value=httpx.Response(
            200,
            json=_schema_response([{"index": 1, "name": "temp", "type": "float"}]),
        )
    )

    fetch_schema("http://test-host", "ch-1", "secret-key")

    assert route.called
    assert route.calls[0].request.headers["X-API-Key"] == "secret-key"


@respx.mock
def test_fetch_schema_non_ok_status():
    respx.get(SCHEMA_URL).mock(return_value=httpx.Response(401))

    with pytest.raises(RuntimeError, match="status 401"):
        fetch_schema("http://test-host", "ch-1", "bad-key")


@respx.mock
def test_fetch_schema_success_false():
    respx.get(SCHEMA_URL).mock(
        return_value=httpx.Response(200, json={"success": False})
    )

    with pytest.raises(RuntimeError, match="failure"):
        fetch_schema("http://test-host", "ch-1", "key")


def test_build_schema_orders_by_index():
    """Fields should be sorted by index regardless of input order."""
    data = {
        "fields": [
            {"index": 3, "name": "co2", "type": "float"},
            {"index": 1, "name": "temp", "type": "float"},
            {"index": 2, "name": "humidity", "type": "float"},
        ],
        "schema_version": 5,
    }

    schema = _build_schema(data)

    assert [f.index for f in schema.ordered_fields] == [1, 2, 3]
    assert [f.name for f in schema.ordered_fields] == ["temp", "humidity", "co2"]
    assert schema.schema_version == 5


def test_build_schema_empty_fields():
    data = {"fields": [], "schema_version": 0}
    schema = _build_schema(data)
    assert schema.ordered_fields == []
    assert schema.name_to_index == {}
