from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal


class DaimonError(Exception):
    """Raised when the sidecar returns a stream error chunk."""


@dataclass
class MemoryResult:
    """A single result returned by a vector store query."""

    id: str
    content: str
    metadata: dict[str, str] = field(default_factory=dict)
    score: float = 0.0

    @classmethod
    def _from_dict(cls, d: dict[str, Any]) -> "MemoryResult":
        meta = d.get("metadata") or {}
        return cls(
            id=d.get("id", ""),
            content=d.get("content", ""),
            metadata={str(k): str(v) for k, v in meta.items()},
            score=float(d.get("score", 0.0)),
        )


@dataclass
class Entity:
    """An extracted entity span from NER."""

    text: str
    label: str
    start: int = 0
    end: int = 0
    confidence: float = 0.0

    @classmethod
    def _from_dict(cls, d: dict[str, Any]) -> "Entity":
        return cls(
            text=d.get("text", ""),
            label=d.get("label", ""),
            start=int(d.get("start", 0)),
            end=int(d.get("end", 0)),
            confidence=float(d.get("confidence", 0.0)),
        )


@dataclass
class ChoiceResult:
    """Result of a decision choose evaluation."""

    selected: str
    index: int
    probabilities: dict[str, float] = field(default_factory=dict)

    @classmethod
    def _from_dict(cls, d: dict[str, Any]) -> "ChoiceResult":
        probs = d.get("probabilities") or {}
        return cls(
            selected=d.get("selected", ""),
            index=int(d.get("index", 0)),
            probabilities={str(k): float(v) for k, v in probs.items()},
        )


@dataclass
class VerifyResult:
    """Result of a decision assertion verification."""

    probability: float
    supported: bool

    @classmethod
    def _from_dict(cls, d: dict[str, Any]) -> "VerifyResult":
        return cls(
            probability=float(d.get("probability", 0.0)),
            supported=bool(d.get("supported", False)),
        )


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
