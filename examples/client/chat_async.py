#!/usr/bin/env python3
"""Async streaming example using daimon.AsyncClient.

Usage:
    pip install daimon-client          # from PyPI
    # pip install -e ../../sdk/python  # local dev
    python chat_async.py [component]
"""

import asyncio
import sys

import daimon_client as daimon


async def main() -> None:
    component = sys.argv[1] if len(sys.argv) > 1 else "llama"
    messages = [
        daimon.Message(role="user", content="What is the capital of France? Answer in one sentence."),
    ]

    async with daimon.AsyncClient() as client:
        print(f"=== {component} (async) ===")
        async for text in client.stream(
            component,
            messages=messages,
            on_tool_call=lambda tc: print(f"\n[calling: {tc.name}]", flush=True),
        ):
            print(text, end="", flush=True)
        print()


if __name__ == "__main__":
    asyncio.run(main())
