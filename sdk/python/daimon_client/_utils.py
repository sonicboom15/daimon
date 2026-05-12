from __future__ import annotations

from typing import Any

from ._types import Message, Tool


def _encode_msg(m: Message | dict[str, Any]) -> dict[str, Any]:
    return m.to_dict() if isinstance(m, Message) else m


def _encode_tool(t: Tool | dict[str, Any]) -> dict[str, Any]:
    return t.to_dict() if isinstance(t, Tool) else t


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
