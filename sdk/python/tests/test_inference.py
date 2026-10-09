"""Unit tests for NERClient, DecisionClient, and their async variants — uses httpx MockTransport."""
from __future__ import annotations

import asyncio
import json
import unittest
from typing import Any

import httpx

from daimon_client import (
    AsyncClient,
    AsyncDecisionClient,
    AsyncNERClient,
    ChoiceResult,
    Client,
    DecisionClient,
    Entity,
    NERClient,
    VerifyResult,
)


class TestEntityAndChoiceTypes(unittest.TestCase):
    def test_entity_from_dict(self):
        e = Entity._from_dict({
            "text": "right knee pain",
            "label": "symptom",
            "start": 10,
            "end": 25,
            "confidence": 0.95,
        })
        self.assertEqual(e.text, "right knee pain")
        self.assertEqual(e.label, "symptom")
        self.assertEqual(e.start, 10)
        self.assertEqual(e.end, 25)
        self.assertAlmostEqual(e.confidence, 0.95)

    def test_entity_from_dict_defaults(self):
        e = Entity._from_dict({"text": "cough", "label": "symptom"})
        self.assertEqual(e.start, 0)
        self.assertEqual(e.end, 0)
        self.assertEqual(e.confidence, 0.0)

    def test_choice_result_from_dict(self):
        r = ChoiceResult._from_dict({
            "selected": "opt-A",
            "index": 0,
            "probabilities": {"opt-A": 0.8, "opt-B": 0.2},
        })
        self.assertEqual(r.selected, "opt-A")
        self.assertEqual(r.index, 0)
        self.assertEqual(r.probabilities["opt-A"], 0.8)
        self.assertEqual(r.probabilities["opt-B"], 0.2)

    def test_verify_result_from_dict(self):
        v = VerifyResult._from_dict({
            "probability": 0.92,
            "supported": True,
        })
        self.assertAlmostEqual(v.probability, 0.92)
        self.assertTrue(v.supported)


class TestNERClientSync(unittest.TestCase):
    def test_extract(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(
                200,
                json={
                    "entities": [
                        {
                            "text": "aspirin",
                            "label": "medication",
                            "start": 5,
                            "end": 12,
                            "confidence": 0.98,
                        }
                    ]
                },
            )

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler))
        ner = client.ner("biomed")
        self.assertIsInstance(ner, NERClient)

        entities = ner.extract(
            "Take aspirin daily",
            labels=["medication", "disease"],
            threshold=0.75,
        )

        self.assertEqual(len(entities), 1)
        self.assertEqual(entities[0].text, "aspirin")
        self.assertEqual(entities[0].label, "medication")
        self.assertEqual(entities[0].start, 5)
        self.assertEqual(entities[0].end, 12)
        self.assertEqual(entities[0].confidence, 0.98)

        self.assertEqual(len(captured), 1)
        self.assertEqual(captured[0].method, "POST")
        self.assertEqual(captured[0].url.path, "/v1/ner/biomed/extract")
        body = json.loads(captured[0].content.decode("utf-8"))
        self.assertEqual(body["text"], "Take aspirin daily")
        self.assertEqual(body["labels"], ["medication", "disease"])
        self.assertEqual(body["threshold"], 0.75)


class TestDecisionClientSync(unittest.TestCase):
    def test_choose(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(
                200,
                json={
                    "selected": "B",
                    "index": 1,
                    "probabilities": {"A": 0.1, "B": 0.9},
                },
            )

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler))
        decision = client.decision("verifier")
        self.assertIsInstance(decision, DecisionClient)

        result = decision.choose(
            question="Which option?",
            choices=["A", "B"],
            state="context string",
        )

        self.assertEqual(result.selected, "B")
        self.assertEqual(result.index, 1)
        self.assertEqual(result.probabilities["B"], 0.9)

        self.assertEqual(len(captured), 1)
        self.assertEqual(captured[0].method, "POST")
        self.assertEqual(captured[0].url.path, "/v1/decision/verifier/choose")
        body = json.loads(captured[0].content.decode("utf-8"))
        self.assertEqual(body["question"], "Which option?")
        self.assertEqual(body["choices"], ["A", "B"])
        self.assertEqual(body["state"], "context string")

    def test_verify(self):
        captured: list[httpx.Request] = []

        def handler(req: httpx.Request) -> httpx.Response:
            captured.append(req)
            return httpx.Response(
                200,
                json={
                    "probability": 0.88,
                    "supported": True,
                },
            )

        client = Client()
        client._http = httpx.Client(transport=httpx.MockTransport(handler))
        decision = client.decision("verifier")

        result = decision.verify(
            statement="The patient has fever.",
            state="Body temp is 38.5C.",
        )

        self.assertTrue(result.supported)
        self.assertAlmostEqual(result.probability, 0.88)

        self.assertEqual(len(captured), 1)
        self.assertEqual(captured[0].method, "POST")
        self.assertEqual(captured[0].url.path, "/v1/decision/verifier/verify")
        body = json.loads(captured[0].content.decode("utf-8"))
        self.assertEqual(body["statement"], "The patient has fever.")
        self.assertEqual(body["state"], "Body temp is 38.5C.")


class TestAsyncInferenceClients(unittest.TestCase):
    def test_async_ner_extract(self):
        async def run():
            captured: list[httpx.Request] = []

            def handler(req: httpx.Request) -> httpx.Response:
                captured.append(req)
                return httpx.Response(
                    200,
                    json={
                        "entities": [
                            {
                                "text": "fever",
                                "label": "symptom",
                                "start": 0,
                                "end": 5,
                                "confidence": 0.99,
                            }
                        ]
                    },
                )

            client = AsyncClient()
            client._http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
            ner = client.ner("clinical")
            self.assertIsInstance(ner, AsyncNERClient)

            entities = await ner.extract("fever and cough")
            self.assertEqual(len(entities), 1)
            self.assertEqual(entities[0].text, "fever")
            self.assertEqual(captured[0].url.path, "/v1/ner/clinical/extract")

        asyncio.run(run())

    def test_async_decision_choose_and_verify(self):
        async def run():
            captured: list[httpx.Request] = []

            def handler(req: httpx.Request) -> httpx.Response:
                captured.append(req)
                if req.url.path.endswith("/choose"):
                    return httpx.Response(
                        200,
                        json={"selected": "Yes", "index": 0, "probabilities": {"Yes": 0.95}},
                    )
                return httpx.Response(200, json={"probability": 0.95, "supported": True})

            client = AsyncClient()
            client._http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
            decision = client.decision("evaluator")
            self.assertIsInstance(decision, AsyncDecisionClient)

            choice = await decision.choose("Is it valid?", ["Yes", "No"])
            self.assertEqual(choice.selected, "Yes")

            verify = await decision.verify("Condition met")
            self.assertTrue(verify.supported)

        asyncio.run(run())


if __name__ == "__main__":
    unittest.main()

