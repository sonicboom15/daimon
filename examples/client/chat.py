#!/usr/bin/env python3
"""Sample client for the daimon sidecar.

Usage:
    pip install requests
    python chat.py                  # runs both components
    python chat.py gpt4o            # run one component
    python chat.py gpt4o claude     # run specific components
"""

import json
import sys

import requests

SIDECAR_URL = "http://127.0.0.1:3500"


def chat(component: str, messages: list[dict], model: str = "") -> str:
    """Stream a chat request to stdout; return the full response text."""
    url = f"{SIDECAR_URL}/v1/converse/{component}"
    payload: dict = {"messages": messages}
    if model:
        payload["model"] = model

    full: list[str] = []
    with requests.post(url, json=payload, stream=True, timeout=120) as resp:
        resp.raise_for_status()
        for raw in resp.iter_lines():
            if not raw:
                continue
            line = raw.decode() if isinstance(raw, bytes) else raw
            if not line.startswith("data: "):
                continue
            chunk = json.loads(line[len("data: "):])
            match chunk["type"]:
                case "text":
                    print(chunk["text"], end="", flush=True)
                    full.append(chunk["text"])
                case "error":
                    print(f"\n[error] {chunk['error']}", file=sys.stderr)
                    break
                case "done":
                    break
    print()
    return "".join(full)


def main() -> None:
    components = sys.argv[1:] or ["gpt4o", "claude"]
    messages = [
        {"role": "user", "content": "What is the capital of France? Answer in one sentence."}
    ]
    for component in components:
        print(f"=== {component} ===")
        chat(component, messages)
        print()


if __name__ == "__main__":
    main()
