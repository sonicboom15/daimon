from __future__ import annotations

import json
from collections.abc import AsyncIterator, Callable
from typing import Any

import httpx

from ._client import _build_body, _normalise_input
from ._types import Chunk, DaimonError, Message, Tool, ToolCall


class AsyncClient:
    """Asynchronous daimon client.

    Use as an async context manager to reuse the underlying HTTP connection,
    or just instantiate directly (a new connection is made per call).

    >>> async with daimon.AsyncClient() as client:
    ...     text = await client.chat("llama", "What is the capital of France?")
    ...     async for chunk in client.converse("claude", messages=[...]):
    ...         if chunk.type == "text":
    ...             print(chunk.text, end="", flush=True)
    """

    def __init__(self, base_url: str = "http://127.0.0.1:3500", timeout: float = 120.0) -> None:
        self._base = base_url.rstrip("/")
        self._timeout = timeout
        self._http: httpx.AsyncClient | None = None

    async def __aenter__(self) -> "AsyncClient":
        self._http = httpx.AsyncClient(timeout=self._timeout)
        return self

    async def __aexit__(self, *_: Any) -> None:
        if self._http:
            await self._http.aclose()
            self._http = None

    def _client(self) -> httpx.AsyncClient:
        if self._http is None:
            self._http = httpx.AsyncClient(timeout=self._timeout)
        return self._http

    async def converse(
        self,
        component: str,
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
        """Async streaming: yields Chunk objects until done or error."""
        body = _build_body(
            messages, model, system, tools, max_tokens, temperature,
            top_p, top_k, stop, frequency_penalty, presence_penalty, seed,
            session_id,
        )
        url = f"{self._base}/v1/converse/{component}"
        async with self._client().stream("POST", url, json=body) as resp:
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
        component: str,
        prompt_or_messages: str | list[Message | dict[str, Any]],
        *,
        on_tool_call: Callable[[ToolCall], None] | None = None,
        model: str = "",
        **kwargs: Any,
    ) -> AsyncIterator[str]:
        """Async text-only stream. Raises DaimonError on error, calls on_tool_call for tool events.

        Pass session_id=... to maintain server-side conversation history.
        """
        messages = _normalise_input(prompt_or_messages)
        async for chunk in self.converse(component, messages=messages, model=model, **kwargs):
            if chunk.type == "text":
                yield chunk.text
            elif chunk.type == "tool_call" and on_tool_call is not None:
                on_tool_call(chunk.tool_call)  # type: ignore[arg-type]
            elif chunk.type == "error":
                raise DaimonError(chunk.error)

    async def chat(
        self,
        component: str,
        prompt_or_messages: str | list[Message | dict[str, Any]],
        *,
        model: str = "",
        **kwargs: Any,
    ) -> str:
        """Async convenience: send and return the full text response.

        Pass session_id=... to maintain server-side conversation history.
        """
        messages = _normalise_input(prompt_or_messages)
        parts: list[str] = []
        async for chunk in self.converse(component, messages=messages, model=model, **kwargs):
            if chunk.type == "text":
                parts.append(chunk.text)
        return "".join(parts)

    async def clear_session(self, session_id: str) -> None:
        """Delete the stored conversation history for the given session ID."""
        resp = await self._client().delete(f"{self._base}/v1/sessions/{session_id}")
        resp.raise_for_status()
