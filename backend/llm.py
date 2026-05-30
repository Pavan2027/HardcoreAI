"""LLM backends for the HardcoreAI agent.

Three providers, one interface. Each `*_complete` coroutine takes the OpenAI-style
`messages` list and returns the full assistant text (no streaming — the agent
loop parses the whole reply at once).

  - llamacpp   — local OpenAI-compatible server (Prism Bonsai 8B, 1-bit quant)
  - openrouter — OpenRouter cloud (gpt-oss-120b)
  - gemini     — Google Gemini API (gemini-2.5-flash)

Keys/URLs come from backend/.env. A provider raises RuntimeError if its key is
missing, so the failure is explicit rather than a confusing 401 later.
"""

from __future__ import annotations

import os

import httpx

# ---------------------------------------------------------------------------
# Provider configuration — read once at import time from the environment.
# ---------------------------------------------------------------------------

LLAMACPP_URL = os.environ.get("LLAMACPP_URL", "http://127.0.0.1:62021").rstrip("/")
LLAMACPP_MODEL = os.environ.get("LLAMACPP_MODEL", "prism-bonsai-8b-1bit")
OPENROUTER_HTTP_REFERER = os.environ.get(
    "OPENROUTER_HTTP_REFERER",
    "http://127.0.0.1:62017",
).strip()

OPENROUTER_API_KEY = os.environ.get("OPENROUTER_API_KEY", "").strip()
OPENROUTER_MODEL = os.environ.get("OPENROUTER_MODEL", "openai/gpt-oss-120b")

GEMINI_API_KEY = os.environ.get("GEMINI_API_KEY", "").strip()
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")

OLLAMA_URL = os.environ.get("OLLAMA_URL", "http://localhost:11434").rstrip("/")
OLLAMA_MODEL = os.environ.get("OLLAMA_MODEL", "qwen2.5-coder:7b")

# Per-provider display metadata, surfaced to the frontend so the panel can list
# what is actually usable (a provider with no key is reported unavailable).
PROVIDERS = {
    "llamacpp": {
        "label": "llama.cpp (Prism Bonsai 8B)",
        "model": LLAMACPP_MODEL,
        "local": True,
    },
    "openrouter": {
        "label": "OpenRouter (gpt-oss-120b)",
        "model": OPENROUTER_MODEL,
        "local": False,
    },
    "gemini": {
        "label": "Gemini 2.5 Flash",
        "model": GEMINI_MODEL,
        "local": False,
    },
    "ollama": {
        "label": "Ollama (local)",
        "model": OLLAMA_MODEL,
        "local": True,
    },
}

# Generation is deterministic-ish and short: the agent only needs a THINK line
# plus one CALL, or a brief final answer.
TEMPERATURE = 0.1
MAX_TOKENS = 4096
HTTP_TIMEOUT = 120.0


class LLMError(RuntimeError):
    """Raised when a provider is misconfigured or the upstream call fails."""


def available_providers() -> list[dict]:
    """Provider descriptors for the frontend, with an `available` flag."""
    out = []
    for key, meta in PROVIDERS.items():
        if key == "openrouter":
            available = bool(OPENROUTER_API_KEY)
        elif key == "gemini":
            available = bool(GEMINI_API_KEY)
        else:  # llamacpp and ollama need no key — availability is "is the server up?",
            available = True  # which we can't know without a probe, so assume yes.
        out.append({"id": key, "available": available, **meta})
    return out


async def _openai_style_complete(
    url: str, model: str, messages: list[dict], headers: dict | None = None
) -> str:
    """POST to any /v1/chat/completions endpoint and return the message text."""
    payload = {
        "model": model,
        "messages": messages,
        "temperature": TEMPERATURE,
        "max_tokens": MAX_TOKENS,
        "stream": False,
    }
    async with httpx.AsyncClient(timeout=HTTP_TIMEOUT) as client:
        try:
            resp = await client.post(url, json=payload, headers=headers or {})
        except httpx.RequestError as exc:
            raise LLMError(f"Failed to connect to {url}. Is the LLM service running? (Error: {exc})") from exc

        if resp.status_code != 200:
            raise LLMError(f"{url} returned {resp.status_code}: {resp.text[:300]}")
        data = resp.json()
    try:
        return data["choices"][0]["message"]["content"] or ""
    except (KeyError, IndexError, TypeError) as exc:
        raise LLMError(f"Unexpected response shape from {url}: {data}") from exc


async def _llamacpp_complete(messages: list[dict]) -> str:
    return await _openai_style_complete(
        f"{LLAMACPP_URL}/v1/chat/completions", LLAMACPP_MODEL, messages
    )


async def _ollama_complete(messages: list[dict]) -> str:
    return await _openai_style_complete(
        f"{OLLAMA_URL}/v1/chat/completions", OLLAMA_MODEL, messages
    )


async def _openrouter_complete(messages: list[dict]) -> str:
    if not OPENROUTER_API_KEY:
        raise LLMError("OPENROUTER_API_KEY is not set in backend/.env.")
    return await _openai_style_complete(
        "https://openrouter.ai/api/v1/chat/completions",
        OPENROUTER_MODEL,
        messages,
        headers={
            "Authorization": f"Bearer {OPENROUTER_API_KEY}",
            "HTTP-Referer": OPENROUTER_HTTP_REFERER,
            "X-Title": "HardcoreAI",
        },
    )


async def _gemini_complete(messages: list[dict]) -> str:
    """Gemini has its own schema — translate the OpenAI message list to it.

    System messages are merged into `system_instruction`; user/assistant turns
    become `contents` with roles user/model.
    """
    if not GEMINI_API_KEY:
        raise LLMError("GEMINI_API_KEY is not set in backend/.env.")

    system_parts = [m["content"] for m in messages if m["role"] == "system"]
    contents = [
        {
            "role": "model" if m["role"] == "assistant" else "user",
            "parts": [{"text": m["content"]}],
        }
        for m in messages
        if m["role"] in ("user", "assistant")
    ]
    payload: dict = {
        "contents": contents,
        "generationConfig": {"temperature": TEMPERATURE, "maxOutputTokens": MAX_TOKENS},
    }
    if system_parts:
        payload["systemInstruction"] = {"parts": [{"text": "\n\n".join(system_parts)}]}

    url = (
        f"https://generativelanguage.googleapis.com/v1beta/models/"
        f"{GEMINI_MODEL}:generateContent"
    )
    async with httpx.AsyncClient(timeout=HTTP_TIMEOUT) as client:
        try:
            resp = await client.post(
                url, json=payload, headers={"x-goog-api-key": GEMINI_API_KEY}
            )
        except httpx.RequestError as exc:
            raise LLMError(f"Failed to connect to Gemini API. (Error: {exc})") from exc
            
        if resp.status_code != 200:
            raise LLMError(f"Gemini returned {resp.status_code}: {resp.text[:300]}")
        data = resp.json()
    try:
        return "".join(
            part.get("text", "")
            for part in data["candidates"][0]["content"]["parts"]
        )
    except (KeyError, IndexError, TypeError) as exc:
        raise LLMError(f"Unexpected Gemini response: {data}") from exc


_DISPATCH = {
    "llamacpp": _llamacpp_complete,
    "ollama": _ollama_complete,
    "openrouter": _openrouter_complete,
    "gemini": _gemini_complete,
}


async def complete(provider: str, messages: list[dict]) -> str:
    """Run a single completion against the named provider."""
    fn = _DISPATCH.get(provider)
    if fn is None:
        raise LLMError(f"Unknown provider '{provider}'. Choose: {list(_DISPATCH)}")
    return await fn(messages)
