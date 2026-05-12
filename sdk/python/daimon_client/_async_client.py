from __future__ import annotations

from collections.abc import AsyncIterator
from typing import Any

import httpx

from ._client import _build_body, _normalise_input
from ._llm_client import AsyncLLMClient
from ._stores import AsyncGraphStoreClient, AsyncMemoryStoreClient
from ._types import Chunk, Message, Tool, ToolCall


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

    def llm(self, component: str = "default") -> AsyncLLMClient:
        """Return an async client scoped to the named LLM component."""
        return AsyncLLMClient(self._base, component, self._client)

    async def converse(
        self,
        component: str = "default",
        *,
        messages: list[Message | dict[str, Any]],
        **kwargs: Any,
    ) -> AsyncIterator[Chunk]:
        """Shorthand for ``client.llm(component).converse(messages=...)``."""
        async for chunk in self.llm(component).converse(messages=messages, **kwargs):
            yield chunk

    async def stream(
        self,
        component: str = "default",
        prompt_or_messages: str | list[Message | dict[str, Any]] = "",
        **kwargs: Any,
    ) -> AsyncIterator[str]:
        """Shorthand for ``client.llm(component).stream(prompt)``."""
        async for text in self.llm(component).stream(prompt_or_messages, **kwargs):
            yield text

    async def chat(
        self,
        component: str = "default",
        prompt_or_messages: str | list[Message | dict[str, Any]] = "",
        **kwargs: Any,
    ) -> str:
        """Shorthand for ``client.llm(component).chat(prompt)``."""
        return await self.llm(component).chat(prompt_or_messages, **kwargs)

    async def clear_session(self, session_id: str) -> None:
        """Shorthand for ``client.llm().clear_session(session_id)``."""
        await self.llm().clear_session(session_id)

    def memory(self, store: str = "default") -> AsyncMemoryStoreClient:
        """Return an async client scoped to the named vector / document store."""
        return AsyncMemoryStoreClient(self._base, store, self._client)

    def graph(self, store: str = "default") -> AsyncGraphStoreClient:
        """Return an async client scoped to the named graph store."""
        return AsyncGraphStoreClient(self._base, store, self._client)
