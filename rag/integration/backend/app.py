from __future__ import annotations

import os
from pathlib import Path

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from .agent_routes import router
from .rag_service import RAGConfig


def _load_env_file() -> None:
    env_path = Path(__file__).with_name(".env")
    if not env_path.exists():
        return

    for raw_line in env_path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        os.environ.setdefault(key.strip(), value.strip())


_load_env_file()

app = FastAPI(
    title="HardcoreAI RAG API",
    version="0.1.0",
    description="FastAPI wrapper around the hardcoreai-rag CLI.",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=[
        "http://localhost:62016",
        "http://localhost:62017",
        "http://127.0.0.1:62016",
        "http://127.0.0.1:62017",
    ],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(router)


@app.get("/")
async def root() -> dict[str, object]:
    config = RAGConfig.from_env()
    return {
        "service": "hardcoreai-rag-fastapi",
        "status": "ready",
        "db_path": str(config.db_path),
        "data_dir": str(config.data_dir),
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        app,
        host=os.environ.get("RAG_API_HOST", "127.0.0.1"),
        port=int(os.environ.get("RAG_API_PORT", "62020")),
    )
