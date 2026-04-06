"""Main Client class for the GreenLab IoT SDK."""

from __future__ import annotations

import threading
from datetime import datetime, timezone
from typing import List, Optional

import httpx

from .batch import Reading, build_batches
from .compress import maybe_compress
from .config import DEFAULT_CONFIG_FILE, LocalConfig, load_config, save_config
from .schema import ChannelSchema, fetch_schema

DEFAULT_BATCH_SIZE = 10
RETRY_TIMEOUT = 5.0  # seconds
CONTENT_TYPE_MSGPACK = "application/msgpack"
HEADER_CONTENT_ENC = "Content-Encoding"
HEADER_RECOMM_FMT = "X-Recommended-Format"
ENCODING_LZ4 = "lz4"


class Client:
    """Sends telemetry readings to a GreenLab ingestion endpoint.

    :param base_url: Base URL of the ingestion service,
        e.g. ``"http://localhost:8003"``.
    :param api_key: Device write API key.
    :param channel_id: UUID of the channel to write to.
    :param batch_size: Maximum number of readings per :meth:`send` call.
        Defaults to 10.
    :param config_file: Path for local config persistence.
        Defaults to ``~/.greenlab/sdk.json``.
    """

    def __init__(
        self,
        base_url: str,
        api_key: str,
        channel_id: str,
        batch_size: int = DEFAULT_BATCH_SIZE,
        config_file: str = DEFAULT_CONFIG_FILE,
    ) -> None:
        if not base_url:
            raise ValueError("sdk: base_url is required")
        if not api_key:
            raise ValueError("sdk: api_key is required")
        if not channel_id:
            raise ValueError("sdk: channel_id is required")

        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        self._channel_id = channel_id
        self._batch_size = batch_size if batch_size > 0 else DEFAULT_BATCH_SIZE
        self._config_file = config_file

        self._lock = threading.Lock()
        self._pending: List[Reading] = []

        # Load persisted config (best-effort).
        try:
            self._local_cfg = load_config(config_file)
        except Exception:
            self._local_cfg = LocalConfig()

        # Fetch schema from server.
        self._schema: ChannelSchema = fetch_schema(base_url, channel_id, api_key)

        # Persist updated config.
        self._local_cfg.channel_id = channel_id
        self._local_cfg.schema_version = self._schema.schema_version
        if not self._local_cfg.format:
            self._local_cfg.format = "msgpack"
        try:
            save_config(config_file, self._local_cfg)
        except Exception:
            pass  # Config persistence is best-effort.

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def set_field(
        self,
        name: str,
        value: float,
        ts: Optional[datetime] = None,
    ) -> None:
        """Record a single field value.

        :param name: Field name as defined in the channel schema.
        :param value: Numeric measurement value.
        :param ts: Timestamp of the measurement. Defaults to ``datetime.now(UTC)``.
        """
        if ts is None:
            ts = datetime.now(timezone.utc)
        with self._lock:
            self._pending.append(Reading(field_name=name, value=value, ts=ts))

    def send(self) -> None:
        """Flush pending readings to the server.

        Handles HTTP 409 (schema version mismatch) by re-fetching the schema
        and retrying once within :data:`RETRY_TIMEOUT` seconds.

        Raises :class:`RuntimeError` on unrecoverable errors.
        """
        with self._lock:
            to_send = self._pending[:]
            self._pending.clear()

        if not to_send:
            return

        try:
            self._send_readings(to_send)
        except _SchemaMismatch:
            # Re-fetch schema and retry once.
            self._schema = fetch_schema(
                self._base_url, self._channel_id, self._api_key
            )
            with self._lock:
                self._local_cfg.schema_version = self._schema.schema_version
            try:
                save_config(self._config_file, self._local_cfg)
            except Exception:
                pass
            self._send_readings(to_send)

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _send_readings(self, readings: List[Reading]) -> None:
        schema = self._schema
        schema_version = self._local_cfg.schema_version
        payloads = build_batches(readings, schema, schema_version)
        for payload in payloads:
            self._post_payload(payload)

    def _post_payload(self, payload: bytes) -> None:
        compressed, did_compress = maybe_compress(payload)
        url = f"{self._base_url}/v1/channels/{self._channel_id}/data"
        headers = {
            "Content-Type": CONTENT_TYPE_MSGPACK,
            "X-API-Key": self._api_key,
        }
        if did_compress:
            headers[HEADER_CONTENT_ENC] = ENCODING_LZ4

        with httpx.Client(timeout=RETRY_TIMEOUT) as http:
            resp = http.post(url, content=compressed, headers=headers)

        # Process X-Recommended-Format header.
        rec_fmt = resp.headers.get(HEADER_RECOMM_FMT)
        if rec_fmt:
            with self._lock:
                self._local_cfg.format = rec_fmt
            try:
                save_config(self._config_file, self._local_cfg)
            except Exception:
                pass

        if resp.status_code == 409:
            raise _SchemaMismatch()
        if resp.status_code >= 300:
            raise RuntimeError(f"sdk: unexpected status {resp.status_code}")


class _SchemaMismatch(Exception):
    """Internal sentinel raised when the server returns HTTP 409."""
