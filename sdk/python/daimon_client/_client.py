from __future__ import annotations

from collections.abc import Iterator
from typing import Any

import httpx

from ._llm_client import LLMClient
from ._stores import GraphStoreClient, MemoryStoreClient
from ._types import Chunk, Message, Tool, ToolCall


def _encode_msg(m: Message | dict[str, Any]) -> dict[str, Any]:
    return m.to_dict() if isinstance(m, Message) else m


def _encode_tool(t: Tool | dict[str, Any]) -> dict[str, Any]:
    return t.to_dict() if isinstance(t, Tool) else t


class Client:
    """Synchronous daimon client.

    Use as a context manager to reuse the underlying HTTP connection across
    calls, or instantiate directly (a new connection is made per call).

    >>> client = daimon.Client()
    >>> text = client.chat("llama", "What is the capital of France?")

    >>> with daimon.Client() as client:
    ...     for text in client.stream("claude", "Hello!"):
    ...         print(text, end="", flush=True)
    """

    def __init__(self, base_url: str = "http://127.0.0.1:3500", timeout: float = 120.0) -> None:
        self._base = base_url.rstrip("/")
        self._timeout = timeout
        self._http: httpx.Client | None = None

    def __enter__(self) -> "Client":
        self._http = httpx.Client(timeout=self._timeout)
        return self

    def __exit__(self, *_: Any) -> None:
        if self._http:
            self._http.close()
            self._http = None

    def _client(self) -> httpx.Client:
        if self._http is None:
            self._http = httpx.Client(timeout=self._timeout)
        return self._http

    def llm(self, component: str = "default") -> LLMClient:
        """Return a client scoped to the named LLM component."""
        return LLMClient(self._base, component, self._client)

    def converse(
        self,
        component: str = "default",
        *,
        messages: list[Message | dict[str, Any]],
        **kwargs: Any,
    ) -> Iterator[Chunk]:
        """Shorthand for ``client.llm(component).converse(messages=...)``. """
        return self.llm(component).converse(messages=messages, **kwargs)

    def stream(
        self,
        component: str = "default",
        prompt_or_messages: str | list[Message | dict[str, Any]] = "",
        **kwargs: Any,
    ) -> Iterator[str]:
        """Shorthand for ``client.llm(component).stream(prompt)``."""
        return self.llm(component).stream(prompt_or_messages, **kwargs)

    def chat(
        self,
        component: str = "default",
        prompt_or_messages: str | list[Message | dict[str, Any]] = "",
        **kwargs: Any,
    ) -> str:
        """Shorthand for ``client.llm(component).chat(prompt)``."""
        return self.llm(component).chat(prompt_or_messages, **kwargs)

    def clear_session(self, session_id: str) -> None:
        """Shorthand for ``client.llm().clear_session(session_id)``."""
        self.llm().clear_session(session_id)

    def memory(self, store: str = "default") -> MemoryStoreClient:
        """Return a client scoped to the named vector / document store."""
        return MemoryStoreClient(self._base, store, self._client)

    def graph(self, store: str = "default") -> GraphStoreClient:
        """Return a client scoped to the named graph store."""
        return GraphStoreClient(self._base, store, self._client)


def _build_body(
    messages: list[Message | dict[str, Any]],
    model: str,
    system: str,
    tools: list[Tool | dict[str, Any]] | None,
    max_tokens: int,
    temperature: float | None,
    top_p: float | None,
    top_k: int | None,
    stop: list[str] | None,
    frequency_penalty: float | None,
    presence_penalty: float | None,
    seed: int | None,
    session_id: str | None = None,
) -> dict[str, Any]:
    body: dict[str, Any] = {"messages": [_encode_msg(m) for m in messages]}
    if model:
        body["model"] = model
    if system:
        body["system"] = system
    if tools:
        body["tools"] = [_encode_tool(t) for t in tools]
    if max_tokens:
        body["max_tokens"] = max_tokens
    if temperature is not None:
        body["temperature"] = temperature
    if top_p is not None:
        body["top_p"] = top_p
    if top_k is not None:
        body["top_k"] = top_k
    if stop:
        body["stop"] = stop
    if frequency_penalty is not None:
        body["frequency_penalty"] = frequency_penalty
    if presence_penalty is not None:
        body["presence_penalty"] = presence_penalty
    if seed is not None:
        body["seed"] = seed
    if session_id:
        body["session_id"] = session_id
    return body


def _normalise_input(
    prompt_or_messages: str | list[Message | dict[str, Any]],
) -> list[Message | dict[str, Any]]:
    if isinstance(prompt_or_messages, str):
        return [{"role": "user", "content": prompt_or_messages}]
    return prompt_or_messages
