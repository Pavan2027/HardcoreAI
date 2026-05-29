"""The HardcoreAI agent: a C-style tool-calling loop over the workbench.

Adapted from the project's tool-calling reference (THINK / CALL protocol, no
JSON). The agent runs in two isolated phases, each with its own fresh context
window:

  Phase 1 — WIRING:  the model sees the problem statement, the placed
                     components and their pins, the current wires, and the
                     component catalogue. It places parts and draws wires.
  Phase 2 — CODING:  a brand-new context. The model sees the same problem plus
                     the *finished* netlist, and writes firmware into src/main.c.

Tools mutate Supabase directly through the same helpers the REST API uses
(`write_workbench`, code-file upsert), so the database stays the single source
of truth and the frontend just re-fetches when the agent finishes.

Why C-style calls instead of JSON: small/local models (the llama.cpp Prism
Bonsai 1-bit quant in particular) produce `CALL name("a", b)` far more reliably
than nested JSON, and it costs fewer tokens.
"""

from __future__ import annotations

import inspect
from dataclasses import dataclass, field
from typing import Any, Callable

# ---------------------------------------------------------------------------
# 1. PARSE ERRORS + C-STYLE CALL PARSER
# ---------------------------------------------------------------------------


class ParseError(Exception):
    """A CALL line could not be parsed into a known tool invocation."""


# Backslash-escape translations inside a quoted string argument.
_ESCAPES = {"n": "\n", "t": "\t", "r": "\r"}


def _tokenize(args_str: str) -> list[Any]:
    """Split a C-style arg list into Python values.

    Handles "double-quoted strings" with backslash escapes, bare tokens
    separated by commas, and coerces bare ints/floats/bools/None. A `key: value`
    prefix (some models emit named args) is stripped down to the value.
    """
    tokens: list[Any] = []
    i, n = 0, len(args_str)
    while i < n:
        while i < n and args_str[i] in " \t,":
            i += 1
        if i >= n:
            break
        if args_str[i] == '"':
            # Check for triple quotes
            is_triple = False
            if i + 2 < n and args_str[i+1] == '"' and args_str[i+2] == '"':
                is_triple = True
                i += 3
            else:
                i += 1
            
            parts: list[str] = []
            while i < n:
                if is_triple:
                    if args_str[i] == '"' and i + 2 < n and args_str[i+1] == '"' and args_str[i+2] == '"':
                        i += 3
                        break
                else:
                    if args_str[i] == '"':
                        i += 1
                        break
                
                if args_str[i] == "\\" and i + 1 < n:
                    if is_triple:
                        parts.append("\\")
                        parts.append(args_str[i+1])
                    else:
                        parts.append(_ESCAPES.get(args_str[i + 1], args_str[i + 1]))
                    i += 2
                else:
                    parts.append(args_str[i])
                    i += 1
            tokens.append("".join(parts))
        else:
            start = i
            depth = 0
            # Bare token runs until a top-level comma or close paren.
            while i < n:
                ch = args_str[i]
                if ch in "([{":
                    depth += 1
                elif ch in ")]}":
                    if depth == 0:
                        break
                    depth -= 1
                elif ch == "," and depth == 0:
                    break
                i += 1
            raw = args_str[start:i].strip()
            colon = raw.find(":")
            if colon != -1:
                key = raw[:colon].strip()
                if key and " " not in key and not key[:1].isdigit():
                    raw = raw[colon + 1:].strip().strip('"').strip("'")
            tokens.append(_coerce(raw))
    return tokens


def _coerce(raw: str) -> Any:
    """Best-effort cast of a bare token to bool/int/float/None, else keep str."""
    if len(raw) >= 2 and raw[1] == ":" and raw[0] in "sifb":
        raw = raw[2:]  # strip type prefixes some models echo: s:foo i:5
    
    # Strip any enclosing quotes for string values
    if len(raw) >= 2 and raw[0] == raw[-1] and raw[0] in "\"'":
        raw = raw[1:-1]
        
    low = raw.lower()
    if low in ("null", "none"):
        return None
    if low in ("false",):
        return False
    if low == "true":
        return True
    try:
        return int(raw)
    except ValueError:
        pass
    try:
        return float(raw)
    except ValueError:
        pass
    return raw


# ---------------------------------------------------------------------------
# 2. TOOL REGISTRY
#
# Tools are plain methods on a Toolbox subclass. Each is wrapped by @tool, which
# records its name, docstring, and parameter signature (read via `inspect`).
# Methods are bound to a toolbox instance per run, so a tool can touch the DB
# session and the project it was constructed with.
# ---------------------------------------------------------------------------


@dataclass
class ToolSpec:
    name: str
    description: str
    params: list[tuple[str, str]]  # (name, type-name) in declaration order
    func: Callable
    wants_body: bool = False  # true if the tool consumes post-CALL fenced text


def tool(func: Callable = None, *, wants_body: bool = False) -> Callable:
    """Mark a Toolbox method as an LLM-callable tool.

    The C-like signature shown to the model is derived from the annotations;
    `self` is skipped. The docstring's first line becomes the description.

    `wants_body=True` declares that the tool consumes the text that follows its
    CALL line (the ``` fence pairs of the Paired wire format). The agent loop
    hands that text to the toolbox before invoking the tool — see run_phase.
    """
    if func is None:  # called as @tool(wants_body=True)
        return lambda f: _make_tool(f, wants_body)
    return _make_tool(func, wants_body)


def _make_tool(func: Callable, wants_body: bool) -> Callable:
    sig = inspect.signature(func)
    params: list[tuple[str, str]] = []
    for pname, p in sig.parameters.items():
        if pname == "self":
            continue
        ann = p.annotation
        # `from __future__ import annotations` makes annotations strings, so an
        # annotation is either a type object (has __name__) or already a string.
        if ann is inspect.Parameter.empty:
            type_name = "str"
        elif isinstance(ann, str):
            type_name = ann
        else:
            type_name = getattr(ann, "__name__", "str")
        if type_name not in ("str", "int", "float", "bool"):
            type_name = "str"
        params.append((pname, type_name))
    func._tool_spec = ToolSpec(  # type: ignore[attr-defined]
        name=func.__name__,
        description=(func.__doc__ or "").strip().splitlines()[0].strip(),
        params=params,
        func=func,
        wants_body=wants_body,
    )
    return func


def build_tool_block(specs: list[ToolSpec]) -> str:
    """Render tool specs as C-like signatures for the system prompt."""
    lines = []
    for spec in specs:
        sig = ", ".join(f"{n}:{t}" for n, t in spec.params)
        lines.append(f"  {spec.name}({sig})  // {spec.description}")
    return "\n".join(lines)


import re

def parse_call(
    raw: str, specs_by_name: dict[str, ToolSpec]
) -> tuple[str, str, dict[str, Any], str] | None:
    """Scan a model reply for the first THINK/CALL pair using regex.

    Returns (thought, tool_name, kwargs, body), or None when no parsable CALL is
    present - in which case `raw` is the model's final answer for the phase.
    """
    call_match = re.search(r"CALL\s+([a-zA-Z0-9_]+)\s*\(", raw, flags=re.IGNORECASE)
    if not call_match:
        # Check for model-specific sentinel
        sentinel_idx = raw.find("<|tool_call>")
        if sentinel_idx == -1:
            return None
        call_str_raw = raw[sentinel_idx:].split("<|tool_call>", 1)[1].lstrip("call:Call ").strip()
        call_idx = sentinel_idx
    else:
        call_idx = call_match.start()
        # Find the end of the line containing the CALL, or the end of the parenthesis
        call_str_raw = raw[call_idx + 4:].lstrip(":= ").strip()

    # Find the matching closing parenthesis for the call
    open_idx = call_str_raw.find("(")
    if open_idx != -1:
        depth = 0
        end_idx = -1
        in_quote = False
        is_triple = False
        i = open_idx
        n = len(call_str_raw)
        while i < n:
            ch = call_str_raw[i]
            if ch == '"':
                if i + 2 < n and call_str_raw[i+1] == '"' and call_str_raw[i+2] == '"':
                    is_triple = not is_triple
                    i += 2
                elif not is_triple:
                    in_quote = not in_quote
            elif ch == "\\" and (in_quote or is_triple):
                i += 1
            elif not in_quote and not is_triple:
                if ch == "(":
                    depth += 1
                elif ch == ")":
                    depth -= 1
                    if depth == 0:
                        end_idx = i
                        break
            i += 1
        
        if end_idx != -1:
            call_str = call_str_raw[:end_idx + 1]
            body = call_str_raw[end_idx + 1:].strip()
        else:
            call_str = call_str_raw
            body = ""
    else:
        call_str = call_str_raw
        body = ""

    # Extract thought
    pre_call = raw[:call_idx]
    think_match = re.search(r"THINK:?\s*(.*)", pre_call, flags=re.IGNORECASE | re.DOTALL)
    thought = think_match.group(1).strip() if think_match else ""

    name, kwargs = _parse_one(call_str, specs_by_name)
    
    return thought, name, kwargs, body


def _parse_one(
    call_str: str, specs_by_name: dict[str, ToolSpec]
) -> tuple[str, dict[str, Any]]:
    """Parse a single `name(arg1, arg2, ...)` against a tool's parameter order."""
    open_idx = call_str.find("(")
    if open_idx == -1:
        raise ParseError(f"no '(' in call: {call_str}")
    name = call_str[:open_idx].strip()
    if name not in specs_by_name:
        raise ParseError(f"unknown tool: {name}")
    if not call_str.rstrip().endswith(")"):
        raise ParseError(f"no closing ')' in call: {call_str}")

    args_str = call_str[open_idx + 1:call_str.rstrip().rfind(")")].strip()
    tokens = _tokenize(args_str) if args_str else []

    spec = specs_by_name[name]
    if len(tokens) > len(spec.params):
        tokens = tokens[:len(spec.params)]
    kwargs: dict[str, Any] = {}
    for (pname, ptype), value in zip(spec.params, tokens):
        kwargs[pname] = _cast(value, ptype)
    return name, kwargs


def _cast(value: Any, type_name: str) -> Any:
    """Coerce a parsed token to the parameter's declared type, leniently."""
    try:
        if type_name == "int":
            return int(value)
        if type_name == "float":
            return float(value)
        if type_name == "bool":
            return value if isinstance(value, bool) else str(value).lower() in ("1", "true", "yes")
        return str(value)
    except (TypeError, ValueError):
        return value


# ---------------------------------------------------------------------------
# 3. AGENT LOOP
# ---------------------------------------------------------------------------

MAX_STEPS = 16


@dataclass
class AgentTrace:
    """A record of one agent run, returned to the frontend for display."""

    phase: str
    steps: list[dict] = field(default_factory=list)
    final: str = ""

    def log_think_call(self, step: int, thought: str, name: str, kwargs: dict, result: str) -> None:
        self.steps.append(
            {"step": step, "think": thought, "call": name, "args": kwargs, "result": result}
        )

    def log_note(self, message: str) -> None:
        self.steps.append({"step": -1, "note": message})


async def run_phase(
    *,
    phase: str,
    system_prompt: str,
    user_prompt: str,
    toolbox: "Toolbox",
    complete_fn: Callable,
) -> AgentTrace:
    """Drive one isolated THINK/CALL phase to completion.

    `complete_fn(messages)` is an async callable returning the model's text —
    this is `functools.partial(llm.complete, provider)` in practice. The phase
    ends when the model emits a reply with no CALL, or the step cap is hit.
    """
    specs_by_name = {s.name: s for s in toolbox.specs()}
    messages: list[dict] = [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": user_prompt},
    ]
    trace = AgentTrace(phase=phase)

    for step in range(MAX_STEPS):
        raw = await complete_fn(messages)
        print("RAW LLM OUTPUT:", repr(raw))
        
        try:
            parsed = parse_call(raw, specs_by_name)
        except ParseError as exc:
            trace.log_think_call(step, "", "parse_error", {}, f"ERROR: {exc}")
            messages.append({"role": "assistant", "content": raw})
            messages.append({"role": "user", "content": f"TOOL RESULT: ParseError: {exc}. Ensure you format your tool call as CALL tool_name(args)."})
            continue

        if parsed is None:
            # Fallback for models that output bare code blocks in the coding phase
            if trace.phase == "coding":
                import re
                code_match = re.search(r"```[a-zA-Z]*\n(.*?)```", raw, flags=re.DOTALL)
                if code_match and ("include" in raw.lower() or "stm32" in raw.lower()):
                    parsed = ("", "write_file", {"path": "src/main.c"}, code_match.group(0))
            
            if parsed is None and trace.phase == "wiring":
                if "```python" in raw.lower() or "```c" in raw.lower():
                    trace.log_think_call(step, "", "parse_error", {}, "ERROR: Code blocks are not allowed in the wiring phase.")
                    messages.append({"role": "assistant", "content": raw})
                    messages.append({"role": "user", "content": "TOOL RESULT: ERROR: Do not write Python or C code. You must use the provided tools like CALL wire_pins(...) to wire the circuit. When you are completely done wiring, output a plain text summary without any code blocks."})
                    continue
                    
        if parsed is None:
            trace.final = raw.strip()
            return trace

        thought, name, kwargs, body = parsed
        spec = specs_by_name[name]
        # A wants_body tool (file_edit) reads the post-CALL fence text from a
        # side-channel attribute the toolbox exposes, so the tool signature
        # stays a plain method the C-style parser can describe.
        toolbox.call_body = body if spec.wants_body else ""
        try:
            result = spec.func(toolbox, **kwargs)
            result_str = str(result)
        except TypeError as exc:  # arity / signature mismatch from the model
            result_str = f"ERROR: bad arguments for {name}: {exc}"
        except Exception as exc:  # noqa: BLE001 — surface any tool failure to the model
            result_str = f"ERROR: {exc}"

        trace.log_think_call(step, thought, name, kwargs, result_str)

        # Feed the model its own emission plus the tool result, then loop.
        messages.append({"role": "assistant", "content": raw})
        messages.append({"role": "user", "content": f"TOOL RESULT: {result_str}"})

    trace.final = "Reached the step limit before finishing."
    trace.log_note("Step limit reached.")
    return trace


# Imported lazily to avoid a circular import (tools.py imports from agent.py).
from tools import Toolbox  # noqa: E402
