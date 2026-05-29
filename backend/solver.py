"""Two-phase solver: wire the workbench, then write the firmware.

`solve_project` is the entry point the API route calls. It runs phase 1 (wiring)
in one isolated agent context, commits the netlist, then runs phase 2 (coding)
in a brand-new context that only sees the finished netlist. Each phase's THINK/
CALL trace is collected and returned so the frontend can show what happened.

The two contexts are deliberately separate: the coding model never sees the
wiring model's reasoning, only its result. This keeps each window small (good
for the 1-bit local quant) and stops phase-1 noise from derailing phase 2.
"""

from __future__ import annotations

from functools import partial

import llm
from agent import AgentTrace, run_phase
from tools import CodingToolbox, WiringToolbox

# ---------------------------------------------------------------------------
# Prompt templates
# ---------------------------------------------------------------------------

_WIRING_SYSTEM = """\
You are a hardware engineer working in the HardcoreAI workbench. You place
electronic components on a canvas and wire their pins together to satisfy a
problem statement.

You have these tools:
{tools}

PROTOCOL — to use a tool you MUST write exactly two lines:
THINK: <one sentence: what you just learned and what you will do next>
CALL tool_name("string arg", 123)

Rules:
- Always write THINK before every CALL. Never skip THINK.
- Arguments are positional and in the order shown above. Quote every string.
- After a TOOL RESULT, write another THINK then your next CALL.
- Call list_catalogue first to see what you can place, and describe_component
  before wiring so you use real pin names.
- Every project needs the STM32F407 Discovery (or similar F407) as the
  controller unless the problem says otherwise.
- Wire power and ground correctly; route motors through a driver, not straight
  to a battery; put a resistor in series with an LED.
- When the workbench fully satisfies the problem and every needed wire exists,
  STOP: reply with a plain sentence summarising what you built and write NO
  THINK or CALL line.
"""

_WIRING_USER = """\
PROBLEM STATEMENT:
{problem}

CURRENT WORKBENCH:
{workbench}

CURRENT WIRES:
{wires}

Configure the workbench: place any missing components and wire the pins so the
problem is solved. Begin.
"""

_CODING_SYSTEM = """\
You are an embedded firmware engineer. You write STM32F407 HAL C
code for a circuit that has ALREADY been wired on the workbench.

You have these tools:
{tools}

PROTOCOL — to use a tool you MUST write exactly two lines:
THINK: <one sentence: what you just learned and what you will do next>
CALL tool_name("string arg", 123)

Rules:
- Always write THINK before every CALL. Never skip THINK.
- Arguments are positional, in the order shown. Quote every string.
- Call netlist first to see exactly which STM32 pins are wired to what.
- Map STM32 header pin labels to ports: label like "C13" -> GPIOC pin 13,
  "A0" -> GPIOA pin 0, "B12" -> GPIOB pin 12.
- CRITICAL: If the circuit involves a specific sensor, driver, or peripheral IC, you MUST call search_hardware_manuals BEFORE writing code to find its exact register addresses, I2C/SPI commands, and configuration sequence. DO NOT hallucinate datasheet values.

WRITING CODE — tool:
- write_file(path): use to write the code. The file content MUST be provided as a markdown code block immediately on the next line after the CALL statement.

  Example:
  CALL write_file("src/main.c")
  ```c
  #include "stm32f4xx_hal.h"
  // ... rest of the code
  ```
  
  - The firmware MUST use the STM32 HAL framework. Do NOT use the legacy Standard Peripheral Library (SPL). Do NOT include "stm32f10x.h".
  - The firmware must be complete and compilable: MUST include "stm32f4xx_hal.h",
    implement HAL_Init(), clock/GPIO init for every wired pin, and a main loop.
  - CRITICAL RULES FOR C FIRMWARE:
    1. Declare all global variables (like huart1) at the TOP of the file.
    2. Use standard escape sequences for strings (e.g. "\\r\\n"). Do NOT use raw newlines.
    3. You MUST define `void SysTick_Handler(void) {{ HAL_IncTick(); }}` at the bottom of the file so HAL timeouts work.
    4. CRITICAL: Peripherals fail silently if clocks are off. If using UART, you MUST enable its clock (e.g., `__HAL_RCC_USART1_CLK_ENABLE()`) AND its GPIO port clock (e.g., `__HAL_RCC_GPIOA_CLK_ENABLE()`), and set the TX pin to `GPIO_MODE_AF_PP`.
- When src/main.c correctly implements the problem for the given netlist,
  STOP: reply with a plain sentence summarising the firmware and write NO
  THINK or CALL line.
"""

_CODING_USER = """\
PROBLEM STATEMENT:
{problem}

FINISHED NETLIST (the circuit is already wired — do not change it):
{netlist}

PLACED COMPONENTS:
{workbench}

Write the firmware into src/main.c so it implements the problem for this exact
circuit. Begin.
"""


def _summarise_workbench(workbench: dict) -> str:
    comps = workbench.get("placed_components", [])
    if not comps:
        return "(empty)"
    return "\n".join(
        f"  [{c['id']}] {c['display_name']} ({c.get('definition_id')})" for c in comps
    )


def _summarise_wires(workbench: dict) -> str:
    wires = workbench.get("wires", [])
    if not wires:
        return "(none)"
    return "\n".join(
        f"  {w['from']['componentId']}.{w['from']['pinName']} -> "
        f"{w['to']['componentId']}.{w['to']['pinName']}"
        for w in wires
    )


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


async def run_wiring_phase(
    *,
    provider: str,
    project_id: str,
    project_name: str,
    problem: str,
    catalogue: dict,
    workbench: dict,
    user_id: str,
) -> tuple[AgentTrace, dict]:
    """Run phase 1. Returns (trace, mutated-workbench)."""
    toolbox = WiringToolbox(
        project_name=project_name,
        problem=problem,
        catalogue=catalogue,
        workbench=workbench,
        user_id=user_id,
        project_id=project_id,
    )
    system = _WIRING_SYSTEM.format(tools=_tool_block(toolbox))
    user = _WIRING_USER.format(
        problem=problem or "(none given)",
        workbench=_summarise_workbench(workbench),
        wires=_summarise_wires(workbench),
    )
    trace = await run_phase(
        phase="wiring",
        system_prompt=system,
        user_prompt=user,
        toolbox=toolbox,
        complete_fn=partial(llm.complete, provider),
    )
    return trace, toolbox.workbench


async def run_coding_phase(
    *, provider: str, project_name: str, problem: str, catalogue: dict,
    workbench: dict, files: dict, user_id: str, project_id: str
) -> tuple[AgentTrace, dict]:
    """Run phase 2. Returns (trace, mutated-files)."""
    toolbox = CodingToolbox(
        project_name=project_name,
        problem=problem,
        catalogue=catalogue,
        workbench=workbench,
        files=files,
        user_id=user_id,
        project_id=project_id,
    )
    system = _CODING_SYSTEM.format(tools=_tool_block(toolbox))
    user = _CODING_USER.format(
        problem=problem or "(none given)",
        netlist=toolbox.netlist(),
        workbench=_summarise_workbench(workbench),
    )
    trace = await run_phase(
        phase="coding",
        system_prompt=system,
        user_prompt=user,
        toolbox=toolbox,
        complete_fn=partial(llm.complete, provider),
    )
    return trace, toolbox.files


def _tool_block(toolbox) -> str:
    from agent import build_tool_block

    return build_tool_block(toolbox.specs())
