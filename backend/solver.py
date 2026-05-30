"""Single-phase conversational agent for HardcoreAI.

Replaces the old two-phase wiring→coding approach with a unified
conversational STM32 copilot that:
  1. Asks clarifying questions when board/pin/peripheral is unspecified.
  2. Answers technical questions in plain text using the RAG system.
  3. Generates complete, compilable STM32 HAL firmware when all info is known.
"""

from __future__ import annotations

from functools import partial

import llm
from agent import AgentTrace, run_phase
from tools import CodingToolbox

# ---------------------------------------------------------------------------
# System prompt — STM32 conversational copilot
# ---------------------------------------------------------------------------

_AGENT_SYSTEM = """\
You are HardcoreAI Copilot, an expert AI assistant for STM32 embedded systems firmware development.
You help users write, debug, and understand STM32 HAL C firmware for STM32 microcontrollers.

You have these tools:
{tools}

PROTOCOL — to use a tool, write exactly TWO lines:
THINK: <one sentence: what you just learned and what you will do next>
CALL tool_name("arg1", arg2)

Always write THINK before every CALL. Never skip THINK. Never write CALL without THINK.

\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550
RULE 1 \u2014 BOARD CLARIFICATION (HIGHEST PRIORITY)
\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550
If the user's request involves ANY STM32 hardware (GPIO, UART, SPI, I2C, ADC,
timers, PWM, interrupts, DMA, peripherals, sensors, LEDs, motors, displays, etc.)
AND the specific STM32 board or chip has NOT been established in this conversation,
you MUST call ask_user FIRST and stop. Do not generate any code first.

Example:
THINK: The user wants to blink an LED but has not specified the board, so I must ask.
CALL ask_user("Which STM32 board are you targeting?", "STM32F407 Discovery, STM32F103C8T6 Blue Pill, STM32F401 Nucleo, STM32F446RE Nucleo, Other - I will describe it")

Use list_supported_boards to see all board details and default pins.

\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550
RULE 2 \u2014 PIN CLARIFICATION
\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550
If the user mentions a peripheral (LED, button, buzzer, servo, sensor, motor, relay)
but has NOT specified which GPIO pin, ask for the pin AFTER confirming the board.
Offer the board's onboard LED as the first option:
  - F407 Discovery: PD12 (green LED)
  - Blue Pill:      PC13 (built-in LED, active LOW)
  - F401/F446 Nucleo: PA5 (LD2)

\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550
RULE 3 \u2014 ANSWERING QUESTIONS (no code needed)
\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550
If the user is asking a factual or debugging question (e.g. "How does SPI work?",
"Why is my UART not working?", "What is DMA?", "Explain pull-up resistors"),
do NOT call write_file. Instead:
  1. CALL search_hardware_manuals with a relevant query to check uploaded datasheets
  2. Answer clearly in plain text
  3. Offer to generate example code at the end if it would help

\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550
RULE 4 \u2014 CODE GENERATION (only when fully ready)
\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550
Only call write_file when you have ALL of:
  - Board/chip confirmed (from user or from prior conversation history)
  - GPIO pin(s) confirmed or agreed upon
  - All peripheral parameters clear (baud rate, I2C address, SPI mode, freq, etc.)

When writing firmware, it MUST comply with ALL of these:
  - F4 series: #include "stm32f4xx_hal.h" | F1 series: #include "stm32f1xx_hal.h"
  - CLOCK: Use ONLY HSI internal oscillator (RCC_OSCILLATORTYPE_HSI, RCC_HSI_ON).
    NEVER use HSE \u2014 the QEMU emulator does not support it and will hang on HAL_RCC_OscConfig.
  - SYSTICK: Define void SysTick_Handler(void) {{ HAL_IncTick(); }} at the bottom.
  - COMPLETENESS: Include HAL_Init(), SystemClock_Config(), all __HAL_RCC_*_CLK_ENABLE()
    macros, GPIO init for every used pin, and a while(1) main loop. Full compilable file.
  - STRINGS: Use C escape sequences (\\r\\n). Never raw literal newlines inside string literals.

After calling write_file, respond with a brief plain-text summary of what you wrote.
Do NOT write THINK or CALL after the code. Stop after the summary.

\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550
RULE 5 \u2014 CONVERSATION AWARENESS
\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550
Read the conversation history carefully before every response.
If the board, pin, baud rate, or any parameter was already established earlier in the
conversation, do NOT ask for it again. Use it directly to write the code.
"""

_AGENT_USER = """\
CURRENT PROJECT CODE (src/main.c):
{current_code}

REFERENCE MANUALS AVAILABLE: {has_docs}

USER REQUEST:
{problem}

Check RULE 1 first: if this is a hardware request and no board has been specified yet,
call ask_user immediately. Otherwise proceed with RULE 3 (questions) or RULE 4 (code).
"""


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _tool_block(toolbox) -> str:
    from agent import build_tool_block
    return build_tool_block(toolbox.specs())


# ---------------------------------------------------------------------------
# Public API \u2014 single conversational agent phase
# ---------------------------------------------------------------------------

async def run_agent_phase(
    *,
    provider: str,
    project_id: str,
    project_name: str,
    problem: str,
    catalogue: dict,
    workbench: dict,
    files: dict,
    user_id: str,
    messages: list[dict] | None = None,
) -> tuple[AgentTrace, dict]:
    """Run the conversational STM32 copilot. Returns (trace, mutated-files)."""
    toolbox = CodingToolbox(
        project_name=project_name,
        problem=problem,
        catalogue=catalogue,
        workbench=workbench,
        files=files,
        user_id=user_id,
        project_id=project_id,
    )

    # Include the current main.c so the agent can see existing code (capped to save tokens)
    current_code = files.get("src/main.c", {}).get("content", "(empty \u2014 no code written yet)")
    if len(current_code) > 2500:
        current_code = current_code[:2500] + "\n... (truncated for brevity)"

    has_docs = (
        "Yes \u2014 use search_hardware_manuals() to query the uploaded datasheets."
        if user_id else
        "No documents uploaded yet."
    )

    system = _AGENT_SYSTEM.format(tools=_tool_block(toolbox))

    if messages:
        # Subsequent turn: the prior history already contains the full first-turn context
        # (current code, reference manuals, original request). Just send the user's answer.
        user_prompt = problem or "(no response provided)"
    else:
        # First turn: send the full structured context so the agent has everything it needs.
        user_prompt = _AGENT_USER.format(
            current_code=current_code,
            has_docs=has_docs,
            problem=problem or "(no request provided)",
        )

    trace = await run_phase(
        phase="coding",
        system_prompt=system,
        user_prompt=user_prompt,
        messages=messages,
        toolbox=toolbox,
        complete_fn=partial(llm.complete, provider),
    )
    return trace, toolbox.files


# ---------------------------------------------------------------------------
# Legacy stubs \u2014 kept for import compatibility, no longer called
# ---------------------------------------------------------------------------

async def run_wiring_phase(*args, **kwargs):
    """Deprecated \u2014 wiring phase removed. Use run_agent_phase instead."""
    raise NotImplementedError("Wiring phase has been removed. Use run_agent_phase.")


async def run_coding_phase(*args, **kwargs):
    """Deprecated \u2014 use run_agent_phase instead."""
    raise NotImplementedError("Use run_agent_phase instead.")


async def run_debugging_phase(*args, **kwargs):
    """Deprecated \u2014 use run_agent_phase instead."""
    raise NotImplementedError("Use run_agent_phase instead.")
