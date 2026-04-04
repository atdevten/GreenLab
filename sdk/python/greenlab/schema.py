"""Schema fetching and caching for the GreenLab SDK."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Dict, List

import httpx


@dataclass
class FieldEntry:
    index: int
    name: str
    type: str


@dataclass
class ChannelSchema:
    """Cached schema for a channel.

    ``name_to_index`` maps field name → 1-based positional index.
    ``ordered_fields`` lists fields sorted by index (the order they appear
    in the positional ``f`` array sent to the server).
    """

    schema_version: int
    ordered_fields: List[FieldEntry] = field(default_factory=list)
    name_to_index: Dict[str, int] = field(default_factory=dict)
    index_to_name: Dict[int, str] = field(default_factory=dict)


def fetch_schema(base_url: str, channel_id: str, api_key: str) -> ChannelSchema:
    """Fetch the channel schema from the server.

    Calls ``GET {base_url}/v1/channels/{channel_id}/schema`` with the
    ``X-API-Key`` header and parses the response into a :class:`ChannelSchema`.

    Raises :class:`RuntimeError` on non-200 responses or malformed payloads.
    """
    url = f"{base_url}/v1/channels/{channel_id}/schema"
    with httpx.Client() as client:
        resp = client.get(url, headers={"X-API-Key": api_key})

    if resp.status_code != 200:
        raise RuntimeError(f"sdk: schema fetch returned status {resp.status_code}")

    payload = resp.json()
    if not payload.get("success"):
        raise RuntimeError("sdk: schema response reported failure")

    return _build_schema(payload["data"])


def _build_schema(data: dict) -> ChannelSchema:
    """Build a :class:`ChannelSchema` from the raw API ``data`` dict."""
    fields = [
        FieldEntry(index=f["index"], name=f["name"], type=f["type"])
        for f in data.get("fields", [])
    ]
    # Sort by index to guarantee positional array alignment.
    fields.sort(key=lambda f: f.index)

    name_to_index = {f.name: f.index for f in fields}
    index_to_name = {f.index: f.name for f in fields}

    return ChannelSchema(
        schema_version=data.get("schema_version", 0),
        ordered_fields=fields,
        name_to_index=name_to_index,
        index_to_name=index_to_name,
    )
