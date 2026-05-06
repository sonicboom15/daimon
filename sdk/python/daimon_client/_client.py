from __future__ import annotations

import json
from collections.abc import Callable, Iterator
from typing import Any

import httpx

from ._types import Chunk, DaimonError, Message, Tool, ToolCall


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

    def converse(
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
    ) -> Iterator[Chunk]:
        """Stream a chat request. Yields Chunk objects until done or error."""
        body = _build_body(
            messages, model, system, tools, max_tokens, temperature,
            top_p, top_k, stop, frequency_penalty, presence_penalty, seed,
            session_id,
        )
        url = f"{self._base}/v1/converse/{component}"
        with self._client().stream("POST", url, json=body) as resp:
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
        component: str,
        prompt_or_messages: str | list[Message | dict[str, Any]],
        *,
        on_tool_call: Callable[[ToolCall], None] | None = None,
        model: str = "",
        **kwargs: Any,
    ) -> Iterator[str]:
        """Yield text fragments. Raises DaimonError on error, calls on_tool_call for tool events.

        Pass session_id=... to maintain server-side conversation history.

        >>> for text in client.stream("llama", "Hello!"):
        ...     print(text, end="", flush=True)

        >>> for text in client.stream("claude", messages=[...],
        ...                           on_tool_call=lambda tc: print(f"[{tc.name}]")):
        ...     print(text, end="", flush=True)
        """
        messages = _normalise_input(prompt_or_messages)
        for chunk in self.converse(component, messages=messages, model=model, **kwargs):
            if chunk.type == "text":
                yield chunk.text
            elif chunk.type == "tool_call" and on_tool_call is not None:
                on_tool_call(chunk.tool_call)  # type: ignore[arg-type]
            elif chunk.type == "error":
                raise DaimonError(chunk.error)

    def chat(
        self,
        component: str,
        prompt_or_messages: str | list[Message | dict[str, Any]],
        *,
        model: str = "",
        **kwargs: Any,
    ) -> str:
        """Convenience wrapper: send and return the full text response.

        Pass session_id=... to maintain server-side conversation history.
        """
        messages = _normalise_input(prompt_or_messages)
        return "".join(
            chunk.text
            for chunk in self.converse(component, messages=messages, model=model, **kwargs)
            if chunk.type == "text"
        )

    def clear_session(self, session_id: str) -> None:
        """Delete the stored conversation history for the given session ID."""
        resp = self._client().delete(f"{self._base}/v1/sessions/{session_id}")
        resp.raise_for_status()


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
