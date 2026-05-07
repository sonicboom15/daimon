# daimon-client

Python client for [Daimon](https://github.com/sonicboom15/daimon) — a pluggable AI sidecar runtime.

## Installation

```bash
pip install daimon-client
```

## Quick start

```python
from daimon_client import Client

with Client(base_url="http://localhost:3500") as c:
    for chunk in c.stream("my-llm", [{"role": "user", "content": "Hello!"}]):
        print(chunk.text, end="", flush=True)
```

### Async

```python
import asyncio
from daimon_client import AsyncClient

async def main():
    async with AsyncClient(base_url="http://localhost:3500") as c:
        async for chunk in c.stream("my-llm", [{"role": "user", "content": "Hello!"}]):
            print(chunk.text, end="", flush=True)

asyncio.run(main())
```

## Vector store client

```python
from daimon_client import MemoryStoreClient

store = MemoryStoreClient(base_url="http://localhost:3500", store="my-store")
store.upsert("doc1", "The Eiffel Tower is 330 metres tall.", metadata={"source": "wikipedia"})
results = store.query("tall Paris structures", top_k=3)
```

## Links

- [Daimon on GitHub](https://github.com/sonicboom15/daimon)
- [Full documentation](https://github.com/sonicboom15/daimon#readme)
