"""Unit tests for daimon_client.Client — uses httpx mock transport, no real server."""
from __future__ import annotations

import json
from collections.abc import Callable

import httpx
import pytest

from daimon_client import Client
from daimon_client._types import Chunk, DaimonError, ToolCall


def _sse_body(*chunks: dict) -> bytes:
    """Build an SSE response body from a sequence of chunk dicts."""
    lines = [f"data: {json.dumps(c)}\n\n" for c in chunks]
    return "".join(lines).encode()


def _mock_client(body: bytes, status: int = 200) -> Client:
    """Return a Client wired to an httpx mock that always returns body."""
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(status, content=body)

    transport = httpx.MockTransport(handler)
    client = Client()
    client._http = httpx.Client(transport=transport, timeout=5.0)
    return client


class TestChat:
    def test_collects_text_chunks(self):
        body = _sse_body(
            {"type": "text", "text": "Hello"},
            {"type": "text", "text": ", world"},
            {"type": "done"},
        )
        client = _mock_client(body)
        result = client.chat("fake", "hi")
        assert result == "Hello, world"

    def test_ignores_non_text_chunks(self):
        body = _sse_body(
            {"type": "tool_call", "tool_call": {"id": "c1", "name": "fn", "input": {}}},
            {"type": "text", "text": "done"},
            {"type": "done"},
        )
        client = _mock_client(body)
        result = client.chat("fake", "hi")
        assert result == "done"

    def test_stops_after_done(self):
        body = _sse_body(
            {"type": "text", "text": "first"},
            {"type": "done"},
            {"type": "text", "text": "should not appear"},
        )
        client = _mock_client(body)
        result = client.chat("fake", "hi")
        assert result == "first"


class TestStream:
    def test_yields_text_fragments(self):
        body = _sse_body(
            {"type": "text", "text": "A"},
            {"type": "text", "text": "B"},
            {"type": "done"},
        )
        client = _mock_client(body)
        parts = list(client.stream("fake", "hi"))
        assert parts == ["A", "B"]

    def test_raises_on_error_chunk(self):
        body = _sse_body({"type": "error", "error": "boom"})
        client = _mock_client(body)
        with pytest.raises(DaimonError, match="boom"):
            list(client.stream("fake", "hi"))

    def test_on_tool_call_callback(self):
        tool_calls: list[ToolCall] = []
        body = _sse_body(
            {"type": "tool_call", "tool_call": {"id": "c1", "name": "search", "input": {}}},
            {"type": "text", "text": "result"},
            {"type": "done"},
        )
        client = _mock_client(body)
        list(client.stream("fake", "hi", on_tool_call=tool_calls.append))
        assert len(tool_calls) == 1
        assert tool_calls[0].name == "search"

    def test_no_callback_skips_tool_calls(self):
        body = _sse_body(
            {"type": "tool_call", "tool_call": {"id": "c1", "name": "fn", "input": {}}},
            {"type": "text", "text": "ok"},
            {"type": "done"},
        )
        client = _mock_client(body)
        parts = list(client.stream("fake", "hi"))
        assert parts == ["ok"]


class TestConverse:
    def test_yields_all_chunk_types(self):
        body = _sse_body(
            {"type": "text", "text": "hi"},
            {"type": "done"},
        )
        client = _mock_client(body)
        chunks = list(client.converse("fake", messages=[{"role": "user", "content": "hi"}]))
        types = [c.type for c in chunks]
        assert types == ["text", "done"]

    def test_skips_non_data_lines(self):
        # SSE can include comment lines (": comment") or blank lines.
        raw = b": this is a comment\n\ndata: {\"type\": \"text\", \"text\": \"hi\"}\n\ndata: {\"type\": \"done\"}\n\n"
        client = _mock_client(raw)
        chunks = list(client.converse("fake", messages=[{"role": "user", "content": "hi"}]))
        assert chunks[0].type == "text"

    def test_stops_after_error_chunk(self):
        body = _sse_body(
            {"type": "error", "error": "fail"},
            {"type": "text", "text": "should not appear"},
        )
        client = _mock_client(body)
        chunks = list(client.converse("fake", messages=[{"role": "user", "content": "hi"}]))
        assert len(chunks) == 1
        assert chunks[0].type == "error"


class TestSession:
    def test_session_id_included_in_request_body(self):
        captured: list[httpx.Request] = []

        def handler(request: httpx.Request) -> httpx.Response:
            captured.append(request)
            return httpx.Response(200, content=_sse_body({"type": "done"}))

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        client.chat("fake", "hi", session_id="sess-abc")

        assert len(captured) == 1
        body = json.loads(captured[0].content)
        assert body["session_id"] == "sess-abc"

    def test_no_session_id_omits_field(self):
        captured: list[httpx.Request] = []

        def handler(request: httpx.Request) -> httpx.Response:
            captured.append(request)
            return httpx.Response(200, content=_sse_body({"type": "done"}))

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        client.chat("fake", "hi")

        body = json.loads(captured[0].content)
        assert "session_id" not in body

    def test_session_id_flows_through_stream(self):
        captured: list[httpx.Request] = []

        def handler(request: httpx.Request) -> httpx.Response:
            captured.append(request)
            return httpx.Response(200, content=_sse_body({"type": "text", "text": "hi"}, {"type": "done"}))

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        list(client.stream("fake", "hello", session_id="sess-xyz"))

        body = json.loads(captured[0].content)
        assert body["session_id"] == "sess-xyz"

    def test_clear_session_sends_delete(self):
        captured: list[httpx.Request] = []

        def handler(request: httpx.Request) -> httpx.Response:
            captured.append(request)
            return httpx.Response(204, content=b"")

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        client.clear_session("sess-abc")

        assert len(captured) == 1
        assert captured[0].method == "DELETE"
        assert captured[0].url.path == "/v1/sessions/sess-abc"


class TestContextManager:
    def test_enter_exit_closes_client(self):
        body = _sse_body({"type": "text", "text": "hi"}, {"type": "done"})

        def handler(request: httpx.Request) -> httpx.Response:
            return httpx.Response(200, content=body)

        with Client() as client:
            client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
            result = client.chat("fake", "hi")
        assert result == "hi"
        assert client._http is None  # closed on __exit__
