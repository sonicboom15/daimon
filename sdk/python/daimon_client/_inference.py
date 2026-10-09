from __future__ import annotations

from collections.abc import Callable
from typing import Any

import httpx

from ._types import ChoiceResult, Entity, VerifyResult


class NERClient:
    """Synchronous client for a single NER component."""

    def __init__(self, base: str, model: str, get_http: Callable[[], httpx.Client]) -> None:
        self._base = base
        self._model = model
        self._get = get_http

    def extract(
        self,
        text: str,
        *,
        labels: list[str] | None = None,
        threshold: float | None = None,
    ) -> list[Entity]:
        """Extract entities and spans from unstructured text."""
        body: dict[str, Any] = {"text": text}
        if labels is not None:
            body["labels"] = labels
        if threshold is not None:
            body["threshold"] = threshold

        resp = self._get().post(f"{self._base}/v1/ner/{self._model}/extract", json=body)
        resp.raise_for_status()
        return [Entity._from_dict(e) for e in resp.json().get("entities", [])]


class AsyncNERClient:
    """Asynchronous client for a single NER component."""

    def __init__(
        self,
        base: str,
        model: str,
        get_http: Callable[[], httpx.AsyncClient],
    ) -> None:
        self._base = base
        self._model = model
        self._get = get_http

    async def extract(
        self,
        text: str,
        *,
        labels: list[str] | None = None,
        threshold: float | None = None,
    ) -> list[Entity]:
        """Extract entities and spans from unstructured text asynchronously."""
        body: dict[str, Any] = {"text": text}
        if labels is not None:
            body["labels"] = labels
        if threshold is not None:
            body["threshold"] = threshold

        resp = await self._get().post(f"{self._base}/v1/ner/{self._model}/extract", json=body)
        resp.raise_for_status()
        return [Entity._from_dict(e) for e in resp.json().get("entities", [])]


class DecisionClient:
    """Synchronous client for a single Decision model."""

    def __init__(self, base: str, model: str, get_http: Callable[[], httpx.Client]) -> None:
        self._base = base
        self._model = model
        self._get = get_http

    def choose(
        self,
        question: str,
        choices: list[str],
        *,
        state: str = "",
    ) -> ChoiceResult:
        """Select the single best match among discrete candidates."""
        body: dict[str, Any] = {
            "question": question,
            "choices": choices,
        }
        if state:
            body["state"] = state

        resp = self._get().post(f"{self._base}/v1/decision/{self._model}/choose", json=body)
        resp.raise_for_status()
        return ChoiceResult._from_dict(resp.json())

    def verify(
        self,
        statement: str,
        *,
        state: str = "",
    ) -> VerifyResult:
        """Evaluate calibrated truth probability for a boolean assertion."""
        body: dict[str, Any] = {"statement": statement}
        if state:
            body["state"] = state

        resp = self._get().post(f"{self._base}/v1/decision/{self._model}/verify", json=body)
        resp.raise_for_status()
        return VerifyResult._from_dict(resp.json())


class AsyncDecisionClient:
    """Asynchronous client for a single Decision model."""

    def __init__(
        self,
        base: str,
        model: str,
        get_http: Callable[[], httpx.AsyncClient],
    ) -> None:
        self._base = base
        self._model = model
        self._get = get_http

    async def choose(
        self,
        question: str,
        choices: list[str],
        *,
        state: str = "",
    ) -> ChoiceResult:
        """Select the single best match among discrete candidates asynchronously."""
        body: dict[str, Any] = {
            "question": question,
            "choices": choices,
        }
        if state:
            body["state"] = state

        resp = await self._get().post(f"{self._base}/v1/decision/{self._model}/choose", json=body)
        resp.raise_for_status()
        return ChoiceResult._from_dict(resp.json())

    async def verify(
        self,
        statement: str,
        *,
        state: str = "",
    ) -> VerifyResult:
        """Evaluate calibrated truth probability for a boolean assertion asynchronously."""
        body: dict[str, Any] = {"statement": statement}
        if state:
            body["state"] = state

        resp = await self._get().post(f"{self._base}/v1/decision/{self._model}/verify", json=body)
        resp.raise_for_status()
        return VerifyResult._from_dict(resp.json())

