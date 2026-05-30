<script lang="ts">
  import { onDestroy } from "svelte";
  import { workspaceStore, actions } from "../store";
  import { 
    Play, 
    Square, 
    RotateCcw, 
    Cpu, 
    Clock, 
    FileText,
    Sliders,
    ArrowRight
  } from "lucide-svelte";

  let logContainer: HTMLDivElement;
  let simulatedTimer: number | null = null;

  // Auto-scroll logs
  $: {
    if ($workspaceStore.emulationLogs && logContainer) {
      setTimeout(() => {
        logContainer.scrollTop = logContainer.scrollHeight;
      }, 50);
    }
  }

  function handleStartEmulation() {
    actions.startEmulation();
  }

  function handleStopEmulation() {
    actions.stopEmulation();
  }

  function handleStep() {
    // Manually step one program pointer instruction
    workspaceStore.update(s => {
      const coreRegs = s.registers.find(r => r.name === "Core Registers");
      const currentPC = parseInt(coreRegs?.bits?.find(b => b.name === "PC")?.value.toString() || "0", 16);
      const nextPC = currentPC + 4;
      const updatedRegs = s.registers.map(reg => {
        if (reg.name === "Core Registers") {
          return {
            ...reg,
            bits: reg.bits?.map(bit => {
              if (bit.name === "PC") return { ...bit, value: nextPC };
              return bit;
            })
          };
        }
        return reg;
      });

      return {
        ...s,
        registers: updatedRegs,
        emulationLogs: [
          ...s.emulationLogs,
          `[EMU] [STEP] Executed 1 cycle. PC advanced to 0x${nextPC.toString(16).toUpperCase().padStart(8, '0')}`
        ]
      };
    });
  }

  function handleReset() {
    workspaceStore.update(s => {
      const updatedRegs = s.registers.map(reg => {
        if (reg.name === "Core Registers") {
          return {
            ...reg,
            bits: reg.bits?.map(bit => {
              if (bit.name === "PC") return { ...bit, value: 0x080010AC };
              return bit;
            })
          };
        }
        return reg;
      });

      return {
        ...s,
        registers: updatedRegs,
        emulationLogs: [
          ...s.emulationLogs,
          `[EMU] System core hot-reset. Program Counter vector remapped to Flash Start (0x080010AC).`
        ]
      };
    });
  }

  function updateSensorValue(sensor: "temp" | "voltage" | "current", val: number) {
    actions.updateAnalogSensor(sensor, val);
  }

  onDestroy(() => {
    if (simulatedTimer) {
      clearInterval(simulatedTimer);
    }
  });
</script>

<div class="emulation-container">
  
  <!-- Left Side: Emulation Controls & Slider Telemetry -->
  <div class="emu-controls-col">
    <div class="emu-section-title">
      <Cpu size={13} style="color: var(--accent-cyan);" />
      <span>EMULATOR CONTROLS</span>
    </div>

    <!-- Power & Execution buttons -->
    <div class="emu-buttons-grid">
      {#if !$workspaceStore.emulationRunning}
        <button class="emu-btn play" onclick={handleStartEmulation} title="Run core simulation">
          <Play size={12} fill="currentColor" />
          <span>Start Emu</span>
        </button>
      {:else}
        <button class="emu-btn stop" onclick={handleStopEmulation} title="Pause core execution">
          <Square size={12} fill="currentColor" />
          <span>Halt Emu</span>
        </button>
      {/if}

      <button class="emu-btn step" onclick={handleStep} disabled={$workspaceStore.emulationRunning} title="Single-step instruction">
        <ArrowRight size={12} />
        <span>Step Cycle</span>
      </button>

      <button class="emu-btn reset" onclick={handleReset} title="Hard reset virtual MCU core">
        <RotateCcw size={12} />
        <span>System Reset</span>
      </button>
    </div>

    <!-- Clock Frequency Configurator -->
    <div class="freq-config-row">
      <label for="freq-select">
        <Clock size={11} />
        <span>CORE CLOCK FREQUENCY:</span>
      </label>
      <select 
        id="freq-select" 
        class="emu-select"
        value={$workspaceStore.emulationFrequency}
        onchange={e => {
          actions.changeEmulationFrequency(e.currentTarget.value as any);
          if ($workspaceStore.emulationRunning) {
            handleStopEmulation();
            handleStartEmulation();
          }
        }}
      >
        <option value="1 Hz">1 Hz (Blinky Debug)</option>
        <option value="10 Hz">10 Hz (Slow speed)</option>
        <option value="100 Hz">100 Hz</option>
        <option value="1 kHz">1 kHz</option>
        <option value="10 kHz">10 kHz</option>
        <option value="1 MHz">1 MHz</option>
        <option value="84 MHz">84 MHz (Full Core Speed)</option>
      </select>
    </div>

    <!-- Analog Input Sliders for Telemetry -->
    <div class="emu-section-title" style="margin-top: 14px;">
      <Sliders size={13} style="color: var(--accent-violet);" />
      <span>ANALOG TELEMETRY INPUTS</span>
    </div>

    <div class="sensors-sliders-grid">
      <!-- Temperature Slider -->
      <div class="sensor-slider-cell">
        <span class="sensor-lbl">TEMP (°C)</span>
        <input 
          type="range" 
          min="10" 
          max="80" 
          step="0.5" 
          value={$workspaceStore.analogSensors.temp}
          oninput={e => updateSensorValue("temp", parseFloat(e.currentTarget.value))}
          class="vertical-slider"
        />
        <span class="sensor-val temp">{$workspaceStore.analogSensors.temp.toFixed(1)}°C</span>
      </div>

      <!-- Voltage Slider -->
      <div class="sensor-slider-cell">
        <span class="sensor-lbl">VDD (V)</span>
        <input 
          type="range" 
          min="1.8" 
          max="3.6" 
          step="0.05" 
          value={$workspaceStore.analogSensors.voltage}
          oninput={e => updateSensorValue("voltage", parseFloat(e.currentTarget.value))}
          class="vertical-slider"
        />
        <span class="sensor-val volt">{$workspaceStore.analogSensors.voltage.toFixed(2)}V</span>
      </div>

      <!-- Current Slider -->
      <div class="sensor-slider-cell">
        <span class="sensor-lbl">IDD (mA)</span>
        <input 
          type="range" 
          min="10" 
          max="120" 
          step="1" 
          value={$workspaceStore.analogSensors.current}
          oninput={e => updateSensorValue("current", parseFloat(e.currentTarget.value))}
          class="vertical-slider"
        />
        <span class="sensor-val curr">{$workspaceStore.analogSensors.current.toFixed(1)}mA</span>
      </div>
    </div>
  </div>

  <!-- Right Side: CPU Registers + Emulation Logs -->
  <div class="emu-registers-col">
    <div class="emu-section-title">
      <FileText size={13} style="color: var(--accent-success);" />
      <span>EMULATOR TELEMETRY LOGS</span>
    </div>

    <!-- Scrolling Logs Terminal -->
    <div class="emu-logs-console" bind:this={logContainer}>
      {#each $workspaceStore.emulationLogs as log}
        <div class="emu-log-line {log.includes('halted') ? 'halt' : log.includes('initialized') ? 'init' : ''}">
          {log}
        </div>
      {/each}
    </div>

    <div class="emu-section-title" style="margin-top: 10px;">
      <Sliders size={13} style="color: var(--accent-success);" />
      <span>EMULATED CPU CORE REGISTER FILE</span>
    </div>

    <!-- Registers Display Block -->
    <div class="emu-registers-grid">
      {#each ($workspaceStore.registers.find(r => r.name === "Core Registers")?.bits) || [] as reg}
        <div class="emu-reg-card">
          <span class="reg-name">{reg.name}</span>
          <span class="reg-val">0x{reg.value.toString(16).toUpperCase().padStart(8, "0")}</span>
          <span class="reg-desc">{reg.description}</span>
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .emulation-container {
    display: flex;
    gap: 16px;
    height: 100%;
    width: 100%;
    background: #09090D;
    color: var(--text-light);
    font-family: var(--font-sans);
    box-sizing: border-box;
  }

  .emu-controls-col {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 10px;
    background: #0E0E14;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 12px;
    box-sizing: border-box;
  }

  .emu-registers-col {
    flex: 1.2;
    display: flex;
    flex-direction: column;
    gap: 8px;
    background: #0E0E14;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 12px;
    box-sizing: border-box;
  }

  .emu-section-title {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.7rem;
    font-weight: 700;
    color: var(--text-muted);
    letter-spacing: 0.8px;
    text-transform: uppercase;
    border-bottom: 1px solid var(--border-color);
    padding-bottom: 4px;
  }

  .emu-buttons-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 6px;
  }

  .emu-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    background: #14141E;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    color: var(--text-light);
    font-family: var(--font-sans);
    font-size: 0.72rem;
    font-weight: 500;
    padding: 6px 4px;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .emu-btn:hover {
    background: #1C1C2A;
    border-color: var(--accent-violet);
  }

  .emu-btn.play {
    color: var(--accent-success);
    border-color: rgba(16, 185, 129, 0.2);
  }
  .emu-btn.play:hover {
    background: rgba(16, 185, 129, 0.08);
    border-color: var(--accent-success);
  }

  .emu-btn.stop {
    color: var(--accent-error);
    border-color: rgba(239, 68, 68, 0.2);
  }
  .emu-btn.stop:hover {
    background: rgba(239, 68, 68, 0.08);
    border-color: var(--accent-error);
  }

  .emu-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .freq-config-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: #12121A;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 6px 10px;
  }

  .freq-config-row label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.65rem;
    color: var(--text-muted);
    font-weight: 600;
  }

  .emu-select {
    background: #181826;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    color: var(--text-bright);
    font-size: 0.7rem;
    font-family: var(--font-sans);
    padding: 2px 4px;
    outline: none;
    cursor: pointer;
  }

  .sensors-sliders-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
    background: #111118;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 10px;
    flex-grow: 1;
    align-content: center;
  }

  .sensor-slider-cell {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }

  .sensor-lbl {
    font-size: 0.65rem;
    font-weight: 600;
    color: var(--text-muted);
  }

  .vertical-slider {
    -webkit-appearance: none;
    appearance: none;
    width: 6px;
    height: 90px;
    background: #1C1C2A;
    border-radius: 3px;
    outline: none;
    writing-mode: vertical-lr;
    direction: rtl;
    cursor: pointer;
  }

  .vertical-slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--accent-violet);
    box-shadow: 0 0 6px rgba(139, 92, 246, 0.6);
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .vertical-slider::-webkit-slider-thumb:hover {
    background: var(--accent-violet-hover);
    transform: scale(1.15);
  }

  .sensor-val {
    font-size: 0.68rem;
    font-family: var(--font-mono);
    font-weight: 600;
  }
  .sensor-val.temp { color: #F59E0B; }
  .sensor-val.volt { color: #06B6D4; }
  .sensor-val.curr { color: #10B981; }

  .emu-logs-console {
    background: #07070B;
    border: 1px solid #1A1A24;
    border-radius: var(--radius-sm);
    padding: 8px 10px;
    font-family: var(--font-mono);
    font-size: 0.65rem;
    height: 95px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .emu-log-line {
    color: #94A3B8;
    line-height: 1.3;
    word-break: break-all;
  }

  .emu-log-line.init {
    color: var(--accent-cyan);
  }

  .emu-log-line.halt {
    color: var(--accent-error);
  }

  .emu-registers-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 6px;
    flex-grow: 1;
    overflow-y: auto;
  }

  .emu-reg-card {
    background: #111116;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 6px 8px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .reg-name {
    font-size: 0.62rem;
    font-weight: 700;
    color: var(--accent-violet);
  }

  .reg-val {
    font-size: 0.72rem;
    font-family: var(--font-mono);
    font-weight: 600;
    color: var(--text-bright);
  }

  .reg-desc {
    font-size: 0.58rem;
    color: var(--text-dark);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
