from __future__ import annotations

from collections.abc import Callable, Coroutine
from typing import Any

import httpx

from ._types import MemoryResult


class MemoryStoreClient:
    """Synchronous client for a single vector / document store."""

    def __init__(self, base: str, store: str, get_http: Callable[[], httpx.Client]) -> None:
        self._base = base
        self._store = store
        self._get = get_http

    def upsert(
        self,
        content: str,
        *,
        id: str = "",
        metadata: dict[str, str] | None = None,
    ) -> str:
        """Insert or update a document. Returns the canonical document ID."""
        body: dict[str, Any] = {"content": content}
        if metadata:
            body["metadata"] = metadata
        if id:
            resp = self._get().put(
                f"{self._base}/v1/memory/{self._store}/{id}", json=body
            )
        else:
            resp = self._get().post(f"{self._base}/v1/memory/{self._store}", json=body)
        resp.raise_for_status()
        return resp.json()["id"]

    def query(self, query: str, *, top_k: int = 5) -> list[MemoryResult]:
        """Semantic similarity search. Returns up to top_k results by descending score."""
        resp = self._get().post(
            f"{self._base}/v1/memory/{self._store}/query",
            json={"query": query, "top_k": top_k},
        )
        resp.raise_for_status()
        return [MemoryResult._from_dict(r) for r in resp.json().get("results", [])]

    def delete(self, id: str) -> None:
        """Delete a document by ID. Idempotent."""
        resp = self._get().delete(f"{self._base}/v1/memory/{self._store}/{id}")
        resp.raise_for_status()


class GraphStoreClient:
    """Synchronous client for a single graph store."""

    def __init__(self, base: str, store: str, get_http: Callable[[], httpx.Client]) -> None:
        self._base = base
        self._store = store
        self._get = get_http

    def add_node(
        self,
        *,
        id: str = "",
        labels: list[str] | None = None,
        props: dict[str, Any] | None = None,
    ) -> str:
        """Add or update a node. Returns the canonical node ID."""
        body: dict[str, Any] = {}
        if labels:
            body["labels"] = labels
        if props:
            body["props"] = props
        if id:
            resp = self._get().put(
                f"{self._base}/v1/graph/{self._store}/nodes/{id}", json=body
            )
        else:
            resp = self._get().post(
                f"{self._base}/v1/graph/{self._store}/nodes", json=body
            )
        resp.raise_for_status()
        return resp.json()["id"]

    def add_edge(
        self,
        from_id: str,
        to_id: str,
        rel_type: str,
        *,
        props: dict[str, Any] | None = None,
    ) -> None:
        """Create a directed relationship between two nodes."""
        body: dict[str, Any] = {"from": from_id, "to": to_id, "type": rel_type}
        if props:
            body["props"] = props
        resp = self._get().post(
            f"{self._base}/v1/graph/{self._store}/edges", json=body
        )
        resp.raise_for_status()

    def cypher(
        self,
        query: str,
        *,
        params: dict[str, Any] | None = None,
    ) -> list[dict[str, Any]]:
        """Run a Cypher query. Returns a list of result rows."""
        resp = self._get().post(
            f"{self._base}/v1/graph/{self._store}/cypher",
            json={"query": query, "params": params or {}},
        )
        resp.raise_for_status()
        return resp.json().get("rows", [])

    def delete_node(self, id: str) -> None:
        """Delete a node and all its relationships. Idempotent."""
        resp = self._get().delete(f"{self._base}/v1/graph/{self._store}/nodes/{id}")
        resp.raise_for_status()


# ── Async variants ────────────────────────────────────────────────────────────

class AsyncMemoryStoreClient:
    """Asynchronous client for a single vector / document store."""

    def __init__(
        self,
        base: str,
        store: str,
        get_http: Callable[[], httpx.AsyncClient],
    ) -> None:
        self._base = base
        self._store = store
        self._get = get_http

    async def upsert(
        self,
        content: str,
        *,
        id: str = "",
        metadata: dict[str, str] | None = None,
    ) -> str:
        """Insert or update a document. Returns the canonical document ID."""
        body: dict[str, Any] = {"content": content}
        if metadata:
            body["metadata"] = metadata
        if id:
            resp = await self._get().put(
                f"{self._base}/v1/memory/{self._store}/{id}", json=body
            )
        else:
            resp = await self._get().post(
                f"{self._base}/v1/memory/{self._store}", json=body
            )
        resp.raise_for_status()
        return resp.json()["id"]

    async def query(self, query: str, *, top_k: int = 5) -> list[MemoryResult]:
        """Semantic similarity search. Returns up to top_k results by descending score."""
        resp = await self._get().post(
            f"{self._base}/v1/memory/{self._store}/query",
            json={"query": query, "top_k": top_k},
        )
        resp.raise_for_status()
        return [MemoryResult._from_dict(r) for r in resp.json().get("results", [])]

    async def delete(self, id: str) -> None:
        """Delete a document by ID. Idempotent."""
        resp = await self._get().delete(f"{self._base}/v1/memory/{self._store}/{id}")
        resp.raise_for_status()


class AsyncGraphStoreClient:
    """Asynchronous client for a single graph store."""

    def __init__(
        self,
        base: str,
        store: str,
        get_http: Callable[[], httpx.AsyncClient],
    ) -> None:
        self._base = base
        self._store = store
        self._get = get_http

    async def add_node(
        self,
        *,
        id: str = "",
        labels: list[str] | None = None,
        props: dict[str, Any] | None = None,
    ) -> str:
        """Add or update a node. Returns the canonical node ID."""
        body: dict[str, Any] = {}
        if labels:
            body["labels"] = labels
        if props:
            body["props"] = props
        if id:
            resp = await self._get().put(
                f"{self._base}/v1/graph/{self._store}/nodes/{id}", json=body
            )
        else:
            resp = await self._get().post(
                f"{self._base}/v1/graph/{self._store}/nodes", json=body
            )
        resp.raise_for_status()
        return resp.json()["id"]

    async def add_edge(
        self,
        from_id: str,
        to_id: str,
        rel_type: str,
        *,
        props: dict[str, Any] | None = None,
    ) -> None:
        """Create a directed relationship between two nodes."""
        body: dict[str, Any] = {"from": from_id, "to": to_id, "type": rel_type}
        if props:
            body["props"] = props
        resp = await self._get().post(
            f"{self._base}/v1/graph/{self._store}/edges", json=body
        )
        resp.raise_for_status()

    async def cypher(
        self,
        query: str,
        *,
        params: dict[str, Any] | None = None,
    ) -> list[dict[str, Any]]:
        """Run a Cypher query. Returns a list of result rows."""
        resp = await self._get().post(
            f"{self._base}/v1/graph/{self._store}/cypher",
            json={"query": query, "params": params or {}},
        )
        resp.raise_for_status()
        return resp.json().get("rows", [])

    async def delete_node(self, id: str) -> None:
        """Delete a node and all its relationships. Idempotent."""
        resp = await self._get().delete(
            f"{self._base}/v1/graph/{self._store}/nodes/{id}"
        )
        resp.raise_for_status()
