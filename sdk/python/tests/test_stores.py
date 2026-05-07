"""Unit tests for MemoryStoreClient and GraphStoreClient — uses httpx mock transport."""
from __future__ import annotations

import json
from typing import Any

import httpx
import pytest

from daimon_client import Client, MemoryResult
from daimon_client._stores import GraphStoreClient, MemoryStoreClient


def _mock_client(responses: dict[str, Any]) -> Client:
    """Build a Client whose HTTP calls are answered by the responses dict.

    Keys are "<METHOD> <path>" (e.g. "PUT /v1/memory/docs/doc1").
    Each value is either a dict (200 JSON) or (status, body_dict).
    """
    def handler(req: httpx.Request) -> httpx.Response:
        key = f"{req.method} {req.url.path}"
        entry = responses.get(key)
        if entry is None:
            return httpx.Response(404, json={"error": f"no mock for {key}"})
        if isinstance(entry, tuple):
            status, body = entry
            return httpx.Response(status, json=body)
        return httpx.Response(200, json=entry)

    client = Client()
    client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
    return client


# ── MemoryStoreClient ─────────────────────────────────────────────────────────

class TestMemoryStoreClientUpsert:
    def test_upsert_with_id_uses_put(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(200, json={"id": "doc1"})

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        store = client.memory("docs")

        returned_id = store.upsert("Hello world", id="doc1")
        assert returned_id == "doc1"
        assert captured[0].method == "PUT"
        assert captured[0].url.path == "/v1/memory/docs/doc1"

    def test_upsert_without_id_uses_post(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(200, json={"id": "server-assigned"})

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        store = client.memory("docs")

        returned_id = store.upsert("Hello world")
        assert returned_id == "server-assigned"
        assert captured[0].method == "POST"
        assert captured[0].url.path == "/v1/memory/docs"

    def test_upsert_sends_metadata(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(200, json={"id": "doc1"})

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        store = client.memory("docs")
        store.upsert("text", id="doc1", metadata={"source": "wiki"})

        body = json.loads(captured[0].content)
        assert body["metadata"]["source"] == "wiki"

    def test_upsert_raises_on_error(self):
        client = _mock_client({})
        store = client.memory("docs")
        with pytest.raises(httpx.HTTPStatusError):
            store.upsert("text", id="doc1")


class TestMemoryStoreClientQuery:
    def test_query_returns_results(self):
        client = _mock_client({
            "POST /v1/memory/docs/query": {
                "results": [
                    {"id": "doc1", "content": "Paris", "score": 0.9, "metadata": {"src": "wiki"}},
                    {"id": "doc2", "content": "London", "score": 0.7},
                ]
            }
        })
        store = client.memory("docs")
        results = store.query("cities", top_k=2)

        assert len(results) == 2
        assert isinstance(results[0], MemoryResult)
        assert results[0].id == "doc1"
        assert results[0].score == 0.9
        assert results[0].metadata["src"] == "wiki"
        assert results[1].metadata == {}

    def test_query_sends_top_k(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(200, json={"results": []})

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        store = client.memory("docs")
        store.query("q", top_k=7)

        body = json.loads(captured[0].content)
        assert body["top_k"] == 7

    def test_query_returns_empty_list_on_no_results(self):
        client = _mock_client({"POST /v1/memory/docs/query": {"results": []}})
        store = client.memory("docs")
        assert store.query("nothing") == []


class TestMemoryStoreClientDelete:
    def test_delete_sends_delete_request(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(204)

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        store = client.memory("docs")
        store.delete("doc1")

        assert captured[0].method == "DELETE"
        assert captured[0].url.path == "/v1/memory/docs/doc1"


# ── GraphStoreClient ──────────────────────────────────────────────────────────

class TestGraphStoreClientAddNode:
    def test_add_node_with_id_uses_put(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(200, json={"id": "alice"})

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        graph = client.graph("kg")

        node_id = graph.add_node(id="alice", labels=["Person"], props={"name": "Alice"})
        assert node_id == "alice"
        assert captured[0].method == "PUT"
        assert captured[0].url.path == "/v1/graph/kg/nodes/alice"

    def test_add_node_without_id_uses_post(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(200, json={"id": "generated-id"})

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        graph = client.graph("kg")

        node_id = graph.add_node(labels=["Thing"])
        assert node_id == "generated-id"
        assert captured[0].method == "POST"
        assert captured[0].url.path == "/v1/graph/kg/nodes"

    def test_add_node_sends_labels_and_props(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(200, json={"id": "n1"})

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        graph = client.graph("kg")
        graph.add_node(id="n1", labels=["A", "B"], props={"x": 1})

        body = json.loads(captured[0].content)
        assert body["labels"] == ["A", "B"]
        assert body["props"]["x"] == 1


class TestGraphStoreClientAddEdge:
    def test_add_edge_posts_correct_body(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(204)

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        graph = client.graph("kg")
        graph.add_edge("alice", "bob", "KNOWS", props={"since": "2020"})

        body = json.loads(captured[0].content)
        assert body["from"] == "alice"
        assert body["to"] == "bob"
        assert body["type"] == "KNOWS"
        assert body["props"]["since"] == "2020"
        assert captured[0].method == "POST"
        assert captured[0].url.path == "/v1/graph/kg/edges"


class TestGraphStoreClientCypher:
    def test_cypher_returns_rows(self):
        client = _mock_client({
            "POST /v1/graph/kg/cypher": {"rows": [{"n.name": "Alice"}, {"n.name": "Bob"}]}
        })
        graph = client.graph("kg")
        rows = graph.cypher("MATCH (n:Person) RETURN n.name")
        assert len(rows) == 2
        assert rows[0]["n.name"] == "Alice"

    def test_cypher_sends_params(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(200, json={"rows": []})

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        graph = client.graph("kg")
        graph.cypher("MATCH (n {id: $id}) RETURN n", params={"id": "alice"})

        body = json.loads(captured[0].content)
        assert body["params"]["id"] == "alice"

    def test_cypher_empty_params_default(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(200, json={"rows": []})

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        graph = client.graph("kg")
        graph.cypher("MATCH (n) RETURN n")

        body = json.loads(captured[0].content)
        assert body["params"] == {}


class TestGraphStoreClientDeleteNode:
    def test_delete_node_sends_delete(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(204)

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler), timeout=5.0)
        graph = client.graph("kg")
        graph.delete_node("alice")

        assert captured[0].method == "DELETE"
        assert captured[0].url.path == "/v1/graph/kg/nodes/alice"


# ── MemoryResult deserialization ──────────────────────────────────────────────

class TestMemoryResult:
    def test_from_dict_full(self):
        r = MemoryResult._from_dict(
            {"id": "d1", "content": "hello", "score": 0.85, "metadata": {"k": "v"}}
        )
        assert r.id == "d1"
        assert r.content == "hello"
        assert r.score == 0.85
        assert r.metadata == {"k": "v"}

    def test_from_dict_defaults(self):
        r = MemoryResult._from_dict({"id": "d2", "content": "x"})
        assert r.score == 0.0
        assert r.metadata == {}

    def test_from_dict_handles_missing_fields(self):
        r = MemoryResult._from_dict({})
        assert r.id == ""
        assert r.content == ""
