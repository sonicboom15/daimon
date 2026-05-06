from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal


class DaimonError(Exception):
    """Raised when the sidecar returns a stream error chunk."""


@dataclass
class ToolCall:
    id: str
    name: str
    input: dict[str, Any]

    def to_dict(self) -> dict[str, Any]:
        return {"id": self.id, "name": self.name, "input": self.input}


@dataclass
class Message:
    role: Literal["system", "user", "assistant", "tool"]
    content: str = ""
    tool_calls: list[ToolCall] = field(default_factory=list)
    tool_call_id: str = ""

    def to_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {"role": self.role}
        if self.content:
            d["content"] = self.content
        if self.tool_calls:
            d["tool_calls"] = [tc.to_dict() for tc in self.tool_calls]
        if self.tool_call_id:
            d["tool_call_id"] = self.tool_call_id
        return d


@dataclass
class Tool:
    name: str
    description: str = ""
    input_schema: dict[str, Any] = field(default_factory=lambda: {"type": "object"})

    def to_dict(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "description": self.description,
            "input_schema": self.input_schema,
        }


@dataclass
class Chunk:
    type: Literal["text", "tool_call", "error", "done"]
    text: str = ""
    tool_call: ToolCall | None = None
    error: str = ""

    @classmethod
    def _from_dict(cls, d: dict[str, Any]) -> "Chunk":
        tc: ToolCall | None = None
        raw = d.get("tool_call")
        if isinstance(raw, dict):
            tc = ToolCall(id=raw.get("id", ""), name=raw.get("name", ""), input=raw.get("input") or {})
        return cls(type=d["type"], text=d.get("text", ""), tool_call=tc, error=d.get("error", ""))
