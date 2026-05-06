"""Unit tests for daimon_client._types — no HTTP required."""
from __future__ import annotations

import pytest

from daimon_client._types import Chunk, DaimonError, Message, Tool, ToolCall


class TestChunkFromDict:
    def test_text_chunk(self):
        c = Chunk._from_dict({"type": "text", "text": "Hello"})
        assert c.type == "text"
        assert c.text == "Hello"
        assert c.tool_call is None
        assert c.error == ""

    def test_done_chunk(self):
        c = Chunk._from_dict({"type": "done"})
        assert c.type == "done"
        assert c.text == ""

    def test_error_chunk(self):
        c = Chunk._from_dict({"type": "error", "error": "upstream failure"})
        assert c.type == "error"
        assert c.error == "upstream failure"

    def test_tool_call_chunk(self):
        c = Chunk._from_dict({
            "type": "tool_call",
            "tool_call": {"id": "call-1", "name": "search", "input": {"q": "Paris"}},
        })
        assert c.type == "tool_call"
        assert c.tool_call is not None
        assert c.tool_call.id == "call-1"
        assert c.tool_call.name == "search"
        assert c.tool_call.input == {"q": "Paris"}

    def test_tool_call_missing_input_defaults_to_empty_dict(self):
        c = Chunk._from_dict({
            "type": "tool_call",
            "tool_call": {"id": "call-2", "name": "noop"},
        })
        assert c.tool_call.input == {}

    def test_tool_call_null_is_ignored(self):
        # Server omits tool_call key entirely for non-tool chunks.
        c = Chunk._from_dict({"type": "text", "text": "hi"})
        assert c.tool_call is None

    def test_tool_call_non_dict_value_is_ignored(self):
        # Malformed response: tool_call is not a dict — should not crash.
        c = Chunk._from_dict({"type": "tool_call", "tool_call": "bad"})
        assert c.tool_call is None

    def test_missing_optional_fields_default(self):
        c = Chunk._from_dict({"type": "done"})
        assert c.text == ""
        assert c.error == ""
        assert c.tool_call is None


class TestToolCallToDict:
    def test_round_trip(self):
        tc = ToolCall(id="c1", name="fn", input={"x": 1})
        d = tc.to_dict()
        assert d == {"id": "c1", "name": "fn", "input": {"x": 1}}


class TestMessageToDict:
    def test_basic_user_message(self):
        m = Message(role="user", content="hello")
        d = m.to_dict()
        assert d == {"role": "user", "content": "hello"}

    def test_omits_empty_content(self):
        m = Message(role="assistant")
        d = m.to_dict()
        assert "content" not in d

    def test_tool_calls_included(self):
        tc = ToolCall(id="c1", name="search", input={})
        m = Message(role="assistant", tool_calls=[tc])
        d = m.to_dict()
        assert "tool_calls" in d
        assert d["tool_calls"][0]["name"] == "search"

    def test_tool_call_id_included(self):
        m = Message(role="tool", content="result", tool_call_id="c1")
        d = m.to_dict()
        assert d["tool_call_id"] == "c1"

    def test_omits_empty_tool_calls(self):
        m = Message(role="user", content="hi")
        d = m.to_dict()
        assert "tool_calls" not in d

    def test_omits_empty_tool_call_id(self):
        m = Message(role="user", content="hi")
        d = m.to_dict()
        assert "tool_call_id" not in d


class TestToolToDict:
    def test_all_fields(self):
        t = Tool(name="search", description="web search", input_schema={"type": "object"})
        d = t.to_dict()
        assert d["name"] == "search"
        assert d["description"] == "web search"
        assert d["input_schema"] == {"type": "object"}

    def test_default_schema(self):
        t = Tool(name="noop")
        d = t.to_dict()
        assert d["input_schema"] == {"type": "object"}


class TestDaimonError:
    def test_is_exception(self):
        err = DaimonError("something went wrong")
        assert isinstance(err, Exception)
        assert str(err) == "something went wrong"
