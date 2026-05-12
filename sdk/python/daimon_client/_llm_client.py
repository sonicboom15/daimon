from __future__ import annotations

import json
from collections.abc import AsyncIterator, Callable, Iterator
from typing import Any

import httpx

from ._client import _build_body, _normalise_input
from ._types import Chunk, DaimonError, Message, Tool, ToolCall


class LLMClient:
    """Synchronous client scoped to a single LLM component."""

    def __init__(self, base: str, component: str, get_client: Callable[[], httpx.Client]) -> None:
        self._base = base
        self._component = component
        self._get_client = get_client

    def converse(
        self,
        *,
        messages: list[Message | dict[str, Any]],
        model: str = "",
        system: str = "",
        tools: list[Tool | dict[str, Any]] | None = None,
        max_tokens: int = 0,
        temperature: float | None = None,
        top_p: float | None = None,
        top_k: int | None = None,
        stop: list[str] | None = None,
        frequency_penalty: float | None = None,
        presence_penalty: float | None = None,
        seed: int | None = None,
        session_id: str | None = None,
    ) -> Iterator[Chunk]:
        """Stream raw Chunk objects until done or error."""
        body = _build_body(
            messages, model, system, tools, max_tokens, temperature,
            top_p, top_k, stop, frequency_penalty, presence_penalty, seed,
            session_id,
        )
        url = f"{self._base}/v1/converse/{self._component}"
        with self._get_client().stream("POST", url, json=body) as resp:
            resp.raise_for_status()
            for line in resp.iter_lines():
                if not line.startswith("data: "):
                    continue
                chunk = Chunk._from_dict(json.loads(line[6:]))
                yield chunk
                if chunk.type in ("done", "error"):
                    return

    def stream(
        self,
        prompt_or_messages: str | list[Message | dict[str, Any]],
        *,
        on_tool_call: Callable[[ToolCall], None] | None = None,
        model: str = "",
        **kwargs: Any,
    ) -> Iterator[str]:
        """Yield text fragments. Raises DaimonError on error."""
        messages = _normalise_input(prompt_or_messages)
        for chunk in self.converse(messages=messages, model=model, **kwargs):
            if chunk.type == "text":
                yield chunk.text
            elif chunk.type == "tool_call" and on_tool_call is not None:
                on_tool_call(chunk.tool_call)  # type: ignore[arg-type]
            elif chunk.type == "error":
                raise DaimonError(chunk.error)

    def chat(
        self,
        prompt_or_messages: str | list[Message | dict[str, Any]],
        *,
        model: str = "",
        **kwargs: Any,
    ) -> str:
        """Send and return the full text response."""
        return "".join(self.stream(prompt_or_messages, model=model, **kwargs))

    def clear_session(self, session_id: str) -> None:
        """Delete stored conversation history for the given session ID."""
        resp = self._get_client().delete(f"{self._base}/v1/sessions/{session_id}")
        resp.raise_for_status()


class AsyncLLMClient:
    """Asynchronous client scoped to a single LLM component."""

    def __init__(
        self, base: str, component: str, get_client: Callable[[], httpx.AsyncClient]
    ) -> None:
        self._base = base
        self._component = component
        self._get_client = get_client

    async def converse(
        self,
        *,
        messages: list[Message | dict[str, Any]],
        model: str = "",
        system: str = "",
        tools: list[Tool | dict[str, Any]] | None = None,
        max_tokens: int = 0,
        temperature: float | None = None,
        top_p: float | None = None,
        top_k: int | None = None,
        stop: list[str] | None = None,
        frequency_penalty: float | None = None,
        presence_penalty: float | None = None,
        seed: int | None = None,
        session_id: str | None = None,
    ) -> AsyncIterator[Chunk]:
        """Async stream of raw Chunk objects until done or error."""
        body = _build_body(
            messages, model, system, tools, max_tokens, temperature,
            top_p, top_k, stop, frequency_penalty, presence_penalty, seed,
            session_id,
        )
        url = f"{self._base}/v1/converse/{self._component}"
        async with self._get_client().stream("POST", url, json=body) as resp:
            resp.raise_for_status()
            async for line in resp.aiter_lines():
                if not line.startswith("data: "):
                    continue
                chunk = Chunk._from_dict(json.loads(line[6:]))
                yield chunk
                if chunk.type in ("done", "error"):
                    return

    async def stream(
        self,
        prompt_or_messages: str | list[Message | dict[str, Any]],
        *,
        on_tool_call: Callable[[ToolCall], None] | None = None,
        model: str = "",
        **kwargs: Any,
    ) -> AsyncIterator[str]:
        """Async text-only stream. Raises DaimonError on error."""
        messages = _normalise_input(prompt_or_messages)
        async for chunk in self.converse(messages=messages, model=model, **kwargs):
            if chunk.type == "text":
                yield chunk.text
            elif chunk.type == "tool_call" and on_tool_call is not None:
                on_tool_call(chunk.tool_call)  # type: ignore[arg-type]
            elif chunk.type == "error":
                raise DaimonError(chunk.error)

    async def chat(
        self,
        prompt_or_messages: str | list[Message | dict[str, Any]],
        *,
        model: str = "",
        **kwargs: Any,
    ) -> str:
        """Async send and return the full text response."""
        parts: list[str] = []
        async for text in self.stream(prompt_or_messages, model=model, **kwargs):
            parts.append(text)
        return "".join(parts)

    async def clear_session(self, session_id: str) -> None:
        """Delete stored conversation history for the given session ID."""
        resp = await self._get_client().delete(f"{self._base}/v1/sessions/{session_id}")
        resp.raise_for_status()
