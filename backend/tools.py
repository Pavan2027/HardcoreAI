"""The Toolbox: the concrete tools the agent calls in each phase.

A Toolbox is constructed per agent run with the project, its DB session, the
component catalogue, and a working copy of the workbench. Tools mutate that
working copy (and, in phase 2, code-file content) in memory; the caller commits
once the phase finishes.

Two tool sets:
  WiringToolbox — place/move/rotate/remove components, wire/unwire pins.
  CodingToolbox — inspect the finished netlist, read/write code files.

Both share read-only inspection tools so the model can always re-orient itself.
"""

from __future__ import annotations

import uuid
from typing import Any

import editmatch
from agent import ToolSpec, tool

class AskUserException(Exception):
    """Raised when the agent wants to ask the user a question."""
    def __init__(self, question: str, options: list[str] = None):
        super().__init__(question)
        self.question = question
        self.options = options or []

class ProposePlanException(Exception):
    """Raised when the agent proposes a plan for user approval."""
    def __init__(self, plan: str):
        super().__init__(plan)
        self.plan = plan

# ---------------------------------------------------------------------------
# Base toolbox — shared inspection tools + the working workbench copy.
# ---------------------------------------------------------------------------


class Toolbox:
    """Holds per-run state and exposes tools as bound methods.

    `workbench` is a mutable dict {placed_components, wires, viewport}. `catalogue`
    maps slug -> ComponentDefinition (the pydantic model from main.py). Subclasses
    add phase-specific tools; `specs()` collects every @tool method on the class.
    """

    def __init__(
        self,
        *,
        project_name: str,
        problem: str,
        catalogue: dict[str, Any],
        workbench: dict[str, Any],
        files: dict[str, dict[str, Any]] | None = None,
        user_id: str | None = None,
        project_id: str | None = None,
    ) -> None:
        self.project_name = project_name
        self.problem = problem
        self.catalogue = catalogue
        self.workbench = workbench
        # files: path -> {"language": str, "content": str}
        self.files = files or {}
        # Set by run_phase before invoking a wants_body tool: the verbatim text
        # following the CALL line, from which file_edit parses its ``` fences.
        self.call_body = ""
        self.user_id = user_id
        self.project_id = project_id

    # -- registry --------------------------------------------------------

    @classmethod
    def specs(cls) -> list[ToolSpec]:
        """Every @tool method on this class (and its bases), declaration order."""
        seen: dict[str, ToolSpec] = {}
        for klass in reversed(cls.__mro__):
            for name, member in vars(klass).items():
                spec = getattr(member, "_tool_spec", None)
                if spec is not None:
                    seen[name] = spec
        return list(seen.values())

    # -- helpers (not tools) --------------------------------------------

    def _find_component(self, ref: str) -> dict[str, Any] | None:
        """Resolve a component reference: its id, or its display name (case-insensitive)."""
        ref = str(ref).strip()
        for c in self.workbench["placed_components"]:
            if c["id"] == ref:
                return c
        low = ref.lower()
        matches = [c for c in self.workbench["placed_components"] if c["display_name"].lower() == low]
        if len(matches) == 1:
            return matches[0]
        matches = [c for c in self.workbench["placed_components"] if c.get("definition_id", "").lower() == low]
        return matches[0] if len(matches) == 1 else None

    def _definition(self, slug: str) -> Any | None:
        return self.catalogue.get(slug)

    def _pin_exists(self, component: dict[str, Any], pin_name: str) -> bool:
        definition = self._definition(component.get("definition_id", ""))
        if not definition:
            return False
        return any(p.name == pin_name for p in definition.pins)

    def _describe_component(self, c: dict[str, Any]) -> str:
        definition = self._definition(c.get("definition_id", ""))
        pins = ", ".join(p.name for p in definition.pins) if definition else "?"
        return (
            f"[{c['id']}] {c['display_name']} ({c.get('definition_id')}) "
            f"at ({int(c['x'])},{int(c['y'])}) rot {c.get('rotation', 0)} — pins: {pins}"
        )

    # -- shared inspection tools ----------------------------------------

    @tool
    def ask_user(self, question: str, options: str = "") -> str:
        """Pause execution and ask the user a clarifying question. Use this if the prompt is ambiguous. Options can be a comma-separated list of choices."""
        opts = [o.strip() for o in options.split(",") if o.strip()]
        raise AskUserException(question, opts)

    @tool
    def propose_plan(self, plan: str) -> str:
        """Pause execution and present an implementation plan to the user for approval. Use this after reading the manual but before wiring or coding."""
        raise ProposePlanException(plan)

    @tool
    def show_problem(self) -> str:
        """Re-read the hardware problem statement you must solve."""
        return self.problem or "(no problem statement provided)"

    @tool
    def list_workbench(self) -> str:
        """List every component currently placed on the workbench, with their pins."""
        comps = self.workbench["placed_components"]
        if not comps:
            return "Workbench is empty."
        lines = [self._describe_component(c) for c in comps]
        return f"{len(comps)} component(s):\n" + "\n".join(lines)

    @tool
    def list_wires(self) -> str:
        """List every wire (pin-to-pin connection) currently on the workbench."""
        wires = self.workbench["wires"]
        if not wires:
            return "No wires yet."
        out = []
        for w in wires:
            out.append(
                f"[{w['id']}] {w['from']['componentId']}.{w['from']['pinName']} "
                f"-> {w['to']['componentId']}.{w['to']['pinName']}"
            )
        return f"{len(wires)} wire(s):\n" + "\n".join(out)

    @tool
    def describe_component(self, component: str) -> str:
        """Show one placed component's details and the role of each of its pins."""
        c = self._find_component(component)
        if not c:
            return f"No placed component matches '{component}'. You must pass the numeric ID (e.g. '64') shown in brackets in list_workbench."
        definition = self._definition(c.get("definition_id", ""))
        if not definition:
            return self._describe_component(c)
        pins = "\n".join(
            f"  {p.name} (label '{p.label}', role {p.role}, side {p.side})"
            for p in definition.pins
        )
        return f"{self._describe_component(c)}\nPins:\n{pins}"

    @tool
    def search_hardware_manuals(self, query: str) -> str:
        """Search the user's uploaded reference manuals and datasheets for hardware information."""
        if not self.user_id or not self.project_id:
            return "ERROR: user_id or project_id is not set. Cannot access the project knowledge base."
        from rag_service import RAGService
        try:
            svc = RAGService(user_id=str(self.user_id), project_id=str(self.project_id))
            result = svc.query(query)
            if result.get("returncode") != 0:
                err = result.get('stderr', '').strip()
                if "no such table: chunks" in err:
                    return "No documents have been uploaded to the hardware manual database yet."
                return f"ERROR: RAG query failed: {err}"
            context = result.get("context", "")
            if not isinstance(context, str) or not context.strip():
                return "No relevant information found in the uploaded manuals."
            return context.strip()
        except Exception as e:
            return f"ERROR: Failed to search manuals: {e}"


# ---------------------------------------------------------------------------
# Phase 1 — WIRING toolbox
# ---------------------------------------------------------------------------


class WiringToolbox(Toolbox):
    """Place components and wire their pins to satisfy the problem statement."""

    @tool
    def list_catalogue(self) -> str:
        """List every component type available in the catalogue to place."""
        lines = []
        for slug, definition in self.catalogue.items():
            pins = ", ".join(f"{p.name}:{p.role}" for p in definition.pins)
            lines.append(f"  {slug} — {definition.name} ({definition.category}); pins: {pins}")
        return "Catalogue:\n" + "\n".join(lines)

    @tool
    def place_component(self, slug: str, name: str = "", x: int = 480, y: int = 280) -> str:
        """Place a catalogue component on the workbench. slug from list_catalogue."""
        definition = self._definition(slug)
        if not definition:
            return f"Unknown slug '{slug}'. Use list_catalogue for valid slugs."
        # Clamp to the same 1600x1000 canvas the frontend uses.
        cx = max(0, min(int(x), 1600 - definition.width))
        cy = max(0, min(int(y), 1000 - definition.height))
        instance = {
            "id": f"part-{uuid.uuid4()}",
            "definition_id": slug,
            "display_name": name.strip() or definition.name,
            "x": cx,
            "y": cy,
            "rotation": 0,
            "config": {},
        }
        self.workbench["placed_components"].append(instance)
        return f"Placed {instance['display_name']} as [{instance['id']}] at ({cx},{cy})."

    @tool
    def move_component(self, component: str, x: int, y: int) -> str:
        """Move a placed component to a new (x, y) position on the canvas."""
        c = self._find_component(component)
        if not c:
            return f"No placed component matches '{component}'."
        definition = self._definition(c.get("definition_id", ""))
        w = definition.width if definition else 140
        h = definition.height if definition else 100
        c["x"] = max(0, min(int(x), 1600 - w))
        c["y"] = max(0, min(int(y), 1000 - h))
        return f"Moved {c['display_name']} to ({c['x']},{c['y']})."

    @tool
    def rotate_component(self, component: str) -> str:
        """Rotate a placed component 90 degrees clockwise."""
        c = self._find_component(component)
        if not c:
            return f"No placed component matches '{component}'."
        c["rotation"] = (int(c.get("rotation", 0)) + 90) % 360
        return f"Rotated {c['display_name']} to {c['rotation']} degrees."

    @tool
    def rename_component(self, component: str, name: str) -> str:
        """Give a placed component a clearer instance name."""
        c = self._find_component(component)
        if not c:
            return f"No placed component matches '{component}'."
        old = c["display_name"]
        c["display_name"] = name.strip() or old
        return f"Renamed '{old}' to '{c['display_name']}'."

    @tool
    def remove_component(self, component: str) -> str:
        """Remove a placed component and every wire attached to it."""
        c = self._find_component(component)
        if not c:
            return f"No placed component matches '{component}'."
        cid = c["id"]
        self.workbench["placed_components"] = [
            x for x in self.workbench["placed_components"] if x["id"] != cid
        ]
        before = len(self.workbench["wires"])
        self.workbench["wires"] = [
            w for w in self.workbench["wires"]
            if w["from"]["componentId"] != cid and w["to"]["componentId"] != cid
        ]
        dropped = before - len(self.workbench["wires"])
        return f"Removed {c['display_name']} and {dropped} attached wire(s)."

    @tool
    def add_wire(self, from_component: str, from_pin: str, to_component: str, to_pin: str) -> str:
        """Wire one pin to another. Pin names come from describe_component."""
        a = self._find_component(from_component)
        b = self._find_component(to_component)
        if not a:
            return f"No placed component matches '{from_component}'."
        if not b:
            return f"No placed component matches '{to_component}'."
        if not self._pin_exists(a, from_pin):
            return f"{a['display_name']} has no pin '{from_pin}'. Use describe_component."
        if not self._pin_exists(b, to_pin):
            return f"{b['display_name']} has no pin '{to_pin}'. Use describe_component."
        if a["id"] == b["id"] and from_pin == to_pin:
            return "A pin cannot wire to itself."
        # Reject an exact duplicate (either direction).
        for w in self.workbench["wires"]:
            ends = {
                (w["from"]["componentId"], w["from"]["pinName"]),
                (w["to"]["componentId"], w["to"]["pinName"]),
            }
            if ends == {(a["id"], from_pin), (b["id"], to_pin)}:
                return "Those two pins are already wired together."
        wire = {
            "id": f"wire-{uuid.uuid4()}",
            "from": {"componentId": a["id"], "pinName": from_pin},
            "to": {"componentId": b["id"], "pinName": to_pin},
        }
        self.workbench["wires"].append(wire)
        return (
            f"Wired {a['display_name']}.{from_pin} -> {b['display_name']}.{to_pin} "
            f"as [{wire['id']}]."
        )

    @tool
    def remove_wire(self, wire_id: str) -> str:
        """Delete a wire by its id (shown in list_wires)."""
        before = len(self.workbench["wires"])
        self.workbench["wires"] = [w for w in self.workbench["wires"] if w["id"] != str(wire_id)]
        if len(self.workbench["wires"]) == before:
            return f"No wire with id '{wire_id}'."
        return f"Removed wire {wire_id}."


# ---------------------------------------------------------------------------
# Phase 2 — CODING toolbox
# ---------------------------------------------------------------------------


class CodingToolbox(Toolbox):
    """Inspect the finished netlist and write STM32 firmware into the code files."""

    @tool(wants_body=True)
    def write_file(self, path: str) -> str:
        """Replace a code file's content entirely. Use only for a new file or a full rewrite."""
        content = self.call_body.strip()
        
        # If the LLM included markdown fences, strip them out
        if content.startswith("```"):
            lines = content.split("\n")
            if len(lines) > 1:
                # Remove the first line (e.g. ```c)
                lines = lines[1:]
                # Remove the last line if it's just ```
                if lines and lines[-1].strip() == "```":
                    lines = lines[:-1]
                content = "\n".join(lines).strip()
                
        language = "markdown" if path.endswith(".md") else "c"
        existing = self.files.get(path)
        if existing is not None:
            language = existing.get("language", language)
            
        # Fallback: if the AI forgets the required SysTick_Handler for the STM32 HAL, auto-inject it
        if path.endswith(".c") and "SysTick_Handler" not in content:
            content = content.rstrip() + "\n\nvoid SysTick_Handler(void) {\n    HAL_IncTick();\n}\n"
            
        self.files[path] = {"language": language, "content": content}
        return f"Successfully wrote {len(content)} bytes to {path}."

    def _file_edit_disabled(self, path: str, old: str = "", new: str = "") -> str:
        """Edit part of a file: keep one unchanged context line above and below the change.

        Two ways to call it. Inline, for a short single-line fix:
            CALL file_edit("src/main.c", "old context+line", "new context+line")
        Or paired, best for multi-line edits — put TWO ``` blocks after the CALL,
        first the before block, then the after block (repeat for more sites):
            CALL file_edit("src/main.c")
            ```c
            <context line>
            <original lines>
            <context line>
            ```
            ```c
            <context line>
            <changed lines>
            <context line>
            ```
        The before block must match the file exactly and uniquely — include
        enough surrounding lines that it appears only once.
        """
        meta = self.files.get(path)
        if meta is None:
            return f"No file '{path}'. Use list_files."

        # Inline form takes priority when old/new were given as args; otherwise
        # parse the ``` fence pairs the agent captured after the CALL line.
        if old or new:
            edits = [editmatch.Edit(old=old, new=new)]
        else:
            edits, parse_err = editmatch.parse_edits(self.call_body or "")
            if parse_err is not None:
                return f"ERROR: {parse_err}"

        content, results = editmatch.apply_all(meta["content"], edits)
        applied = [r for r in results if r.applied]
        failed = next((r for r in results if r.error is not None), None)

        if failed is not None:
            # apply_all keeps earlier successes in `content`; persist them so
            # the model sees partial progress, then report the failing site.
            if applied:
                meta["content"] = content
            done = f"{len(applied)} edit(s) applied; " if applied else ""
            return f"ERROR: {done}edit #{len(applied) + 1} failed: {failed.error}"

        meta["content"] = content
        spans = ", ".join(f"L{r.start_line}-{r.end_line}" for r in applied)
        return f"Applied {len(applied)} edit(s) to {path} ({spans})."

    @tool
    def netlist(self) -> str:
        """Show the full netlist: every wire with both endpoints' component + pin role."""
        wires = self.workbench["wires"]
        if not wires:
            return "Netlist is empty — no wires."
        by_id = {c["id"]: c for c in self.workbench["placed_components"]}

        def endpoint(e: dict[str, Any]) -> str:
            c = by_id.get(e["componentId"])
            if not c:
                return f"?.{e['pinName']}"
            definition = self._definition(c.get("definition_id", ""))
            pin = next((p for p in definition.pins if p.name == e["pinName"]), None) if definition else None
            label = pin.label if pin else e["pinName"]
            role = pin.role if pin else "?"
            return f"{c['display_name']}.{label}({role})"

        lines = [f"  {endpoint(w['from'])} <-> {endpoint(w['to'])}" for w in wires]
        return f"Netlist ({len(wires)} connections):\n" + "\n".join(lines)



# ---------------------------------------------------------------------------
# Phase 3 — DEBUGGING toolbox
# ---------------------------------------------------------------------------

import httpx

class DebuggingToolbox(Toolbox):
    """Run the Go emulator to build, flash, and debug the STM32 code."""

    def __init__(self, *args, **kwargs):
        self.files = kwargs.pop("files", {})
        super().__init__(*args, **kwargs)
        self.emulator_url = "http://127.0.0.1:62019"

    @tool
    def build_and_run(self) -> str:
        """Build the firmware from the current code files and run it in the emulator."""
        pio_files = [{"path": p, "content": f["content"]} for p, f in self.files.items()]
        if not any(f["path"] == "platformio.ini" for f in pio_files):
            pio_files.append({
                "path": "platformio.ini",
                "content": "[env:disco_f100rb]\\nplatform = ststm32\\nboard = disco_f100rb\\nframework = stm32cube\\n"
            })
            
        try:
            resp = httpx.post(f"{self.emulator_url}/platformio/build", json={
                "projectPath": "./Blinky",
                "files": pio_files
            }, timeout=30.0)
            if resp.status_code != 200:
                return f"ERROR: Build failed:\\n{resp.text}"
                
            # Now run it
            resp_run = httpx.get(f"{self.emulator_url}/qemu/run", timeout=10.0)
            if resp_run.status_code != 200:
                return f"ERROR: Run failed:\\n{resp_run.text}"
            return "Firmware built and QEMU is running. You can now connect the debugger."
        except Exception as e:
            return f"ERROR: {e}"

    @tool
    def connect_debugger(self) -> str:
        """Connect GDB to the running emulator."""
        try:
            resp = httpx.get(f"{self.emulator_url}/debug/connect", timeout=10.0)
            if resp.status_code != 200:
                return f"ERROR: Connect failed:\\n{resp.text}"
            return resp.text
        except Exception as e:
            return f"ERROR: {e}"

    @tool
    def read_registers(self) -> str:
        """Read CPU registers (PC, SP, R0-R12, etc.)."""
        try:
            resp = httpx.get(f"{self.emulator_url}/debug/registers", timeout=5.0)
            if resp.status_code != 200:
                return f"ERROR: Failed to read registers:\\n{resp.text}"
            return resp.text
        except Exception as e:
            return f"ERROR: {e}"

    @tool
    def step_debugger(self) -> str:
        """Step the CPU by one instruction."""
        try:
            resp = httpx.get(f"{self.emulator_url}/debug/step", timeout=5.0)
            if resp.status_code != 200:
                return f"ERROR: Step failed:\\n{resp.text}"
            return resp.text
        except Exception as e:
            return f"ERROR: {e}"
