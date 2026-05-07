"""End-to-end tests for the Python daimon client against a real daimon sidecar.

Requires:
  - DAIMON_E2E=1        to opt in (skip otherwise)
  - DAIMON_BASE_URL=... to point at the running sidecar

Normally invoked by the Go integration test in test/e2e/e2e_test.go, which
starts an Ollama container and daimon server automatically.  Can also be run
manually against any live sidecar:

    DAIMON_E2E=1 DAIMON_BASE_URL=http://127.0.0.1:3500 \\
        pytest sdk/python/tests/test_e2e.py -v
"""
from __future__ import annotations

import os
import uuid

import pytest

import daimon_client as daimon

# ---------------------------------------------------------------------------
# Guard: skip every test unless explicitly opted in.
# ---------------------------------------------------------------------------

pytestmark = pytest.mark.skipif(
    not os.getenv("DAIMON_E2E"),
    reason="set DAIMON_E2E=1 to run e2e tests",
)

COMPONENT = "llama"
BASE_URL = os.getenv("DAIMON_BASE_URL", "http://127.0.0.1:3500")
MEM_STORE = os.getenv("DAIMON_MEM_STORE", "mem")


@pytest.fixture
def client() -> daimon.Client:
    return daimon.Client(base_url=BASE_URL, timeout=120.0)


# ---------------------------------------------------------------------------
# Basic inference
# ---------------------------------------------------------------------------


class TestChat:
    def test_returns_non_empty_text(self, client: daimon.Client) -> None:
        result = client.chat(COMPONENT, "Reply with exactly one word: PONG")
        assert result.strip() != "", "expected non-empty response"

    def test_stream_yields_fragments(self, client: daimon.Client) -> None:
        parts = list(client.stream(COMPONENT, "Reply with exactly one word: PONG"))
        assert "".join(parts).strip() != "", "expected non-empty streamed text"

    def test_multi_turn_without_session(self, client: daimon.Client) -> None:
        """Client manually carries history — no session involved."""
        messages: list[dict] = [
            {"role": "user", "content": "My favourite colour is red."},
            {"role": "assistant", "content": "Got it, your favourite colour is red."},
            {"role": "user", "content": "What colour did I just tell you is my favourite?"},
        ]
        result = client.chat(COMPONENT, messages)
        assert "red" in result.lower(), f"expected 'red' in response, got: {result!r}"


# ---------------------------------------------------------------------------
# Session management
# ---------------------------------------------------------------------------


class TestSession:
    def test_session_recalls_fact(self, client: daimon.Client) -> None:
        session_id = f"pytest-recall-{uuid.uuid4().hex[:8]}"
        try:
            client.chat(COMPONENT, "My favourite colour is blue.", session_id=session_id)
            reply = client.chat(
                COMPONENT, "What colour did I just tell you is my favourite?", session_id=session_id
            )
            assert "blue" in reply.lower(), f"expected 'blue' in session recall, got: {reply!r}"
        finally:
            client.clear_session(session_id)

    def test_session_accumulates_across_stream_calls(self, client: daimon.Client) -> None:
        session_id = f"pytest-stream-{uuid.uuid4().hex[:8]}"
        try:
            # Use stream() for turn 1 — session_id flows via **kwargs
            list(client.stream(COMPONENT, "My favourite colour is blue.", session_id=session_id))
            reply = client.chat(
                COMPONENT, "What colour did I just tell you is my favourite?", session_id=session_id
            )
            assert "blue" in reply.lower(), f"expected 'blue' in response, got: {reply!r}"
        finally:
            client.clear_session(session_id)

    def test_different_sessions_are_independent(self, client: daimon.Client) -> None:
        sid_a = f"pytest-iso-a-{uuid.uuid4().hex[:8]}"
        sid_b = f"pytest-iso-b-{uuid.uuid4().hex[:8]}"
        try:
            client.chat(COMPONENT, "My favourite colour is orange.", session_id=sid_a)
            client.chat(COMPONENT, "My favourite colour is green.", session_id=sid_b)

            reply_a = client.chat(COMPONENT, "What colour did I just tell you is my favourite?", session_id=sid_a)
            reply_b = client.chat(COMPONENT, "What colour did I just tell you is my favourite?", session_id=sid_b)

            assert "orange" in reply_a.lower(), f"session A expected orange, got: {reply_a!r}"
            assert "green" in reply_b.lower(), f"session B expected green, got: {reply_b!r}"
        finally:
            client.clear_session(sid_a)
            client.clear_session(sid_b)

    def test_clear_session_does_not_raise(self, client: daimon.Client) -> None:
        session_id = f"pytest-clear-{uuid.uuid4().hex[:8]}"
        client.chat(COMPONENT, "Hello.", session_id=session_id)
        client.clear_session(session_id)  # must not raise

    def test_clear_nonexistent_session_does_not_raise(self, client: daimon.Client) -> None:
        client.clear_session(f"pytest-ghost-{uuid.uuid4().hex[:8]}")


# ---------------------------------------------------------------------------
# Memory store (inmemory BM25 — no external service)
# ---------------------------------------------------------------------------


class TestMemoryStore:
    def test_upsert_with_id_returns_id(self, client: daimon.Client) -> None:
        store = client.memory(MEM_STORE)
        doc_id = f"e2e-{uuid.uuid4().hex[:8]}"
        returned = store.upsert("The Eiffel Tower is 330 metres tall.", id=doc_id)
        assert returned == doc_id

    def test_upsert_without_id_returns_assigned_id(self, client: daimon.Client) -> None:
        store = client.memory(MEM_STORE)
        assigned = store.upsert("The Seine is the main river of Paris.")
        assert assigned != "", "expected non-empty assigned id"

    def test_query_returns_seeded_document(self, client: daimon.Client) -> None:
        store = client.memory(MEM_STORE)
        doc_id = f"e2e-query-{uuid.uuid4().hex[:8]}"
        store.upsert("Python is a popular programming language.", id=doc_id)

        results = store.query("programming language", top_k=5)
        assert len(results) > 0, "expected at least one result"
        ids = [r.id for r in results]
        assert doc_id in ids, f"{doc_id!r} not in result ids: {ids}"

        # Verify MemoryResult fields are populated.
        matched = next(r for r in results if r.id == doc_id)
        assert matched.content != ""
        assert isinstance(matched.score, float)

    def test_query_returns_memory_result_instances(self, client: daimon.Client) -> None:
        store = client.memory(MEM_STORE)
        doc_id = f"e2e-type-{uuid.uuid4().hex[:8]}"
        store.upsert("Go is a compiled language.", id=doc_id)

        results = store.query("compiled language", top_k=3)
        assert all(isinstance(r, daimon.MemoryResult) for r in results)

    def test_upsert_with_metadata(self, client: daimon.Client) -> None:
        store = client.memory(MEM_STORE)
        doc_id = f"e2e-meta-{uuid.uuid4().hex[:8]}"
        store.upsert("Rust is a systems language.", id=doc_id, metadata={"src": "wiki"})

        results = store.query("systems language", top_k=5)
        ids = [r.id for r in results]
        assert doc_id in ids

    def test_delete_removes_document(self, client: daimon.Client) -> None:
        store = client.memory(MEM_STORE)
        doc_id = f"e2e-del-{uuid.uuid4().hex[:8]}"
        store.upsert("Temporary document to be deleted.", id=doc_id)
        store.delete(doc_id)  # must not raise

    def test_delete_idempotent(self, client: daimon.Client) -> None:
        store = client.memory(MEM_STORE)
        doc_id = f"e2e-idem-{uuid.uuid4().hex[:8]}"
        store.upsert("Another temporary document.", id=doc_id)
        store.delete(doc_id)
        store.delete(doc_id)  # second delete must not raise
