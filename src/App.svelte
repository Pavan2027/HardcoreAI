<script lang="ts">
  import { tick, onMount } from "svelte";
  import { api } from "./api";
  import { workspaceStore, actions, type FileItem } from "./store";
  import * as monaco from "monaco-editor";
  import EmbeddedConfigurator from "./components/EmbeddedConfigurator.svelte";
  import EmulationPanel from "./components/EmulationPanel.svelte";
  import RagUploadPanel from "./components/RagUploadPanel.svelte";

  import {
    Play,
    Zap,
    Bug,
    FolderOpen,
    FileCode,
    File,
    Send,
    AlertTriangle,
    Sparkles,
    ArrowRight,
    Search,
    GitBranch,
    Blocks,
    Folder,
    Settings,
    X,
    ChevronDown,
    MoreHorizontal,
    Plus,
    Moon,
    Cpu,
    Database,
    Sliders,
    Trash2,
    MonitorPlay,
  } from "lucide-svelte";

  let aiInput = "";
  let serialInput = "";
  let selectedPeripheral = "Core Registers";
  let aiOpen = true;
  let showConfigurator = false;
  let terminalOpen = true;

  // Panel sizing
  let sidebarWidth = 260;
  let rightSidebarWidth = 380;
  let bottomDrawerHeight = 220;

  let isDraggingLeft = false;
  let isDraggingRight = false;
  let isDraggingBottom = false;

  // DOM Elements
  let canvasEl: HTMLCanvasElement;
  let terminalEndRef: HTMLDivElement;
  let monacoEditor: monaco.editor.IStandaloneCodeEditor | null = null;

  async function handleMouseMove(e: MouseEvent) {
    if (isDraggingLeft) {
      sidebarWidth = Math.max(180, Math.min(450, e.clientX - 52));
      await tick();
      window.requestAnimationFrame(() => {
        if (monacoEditor) monacoEditor.layout();
        resetEditorScroll();
      });
    }
    if (isDraggingRight) {
      rightSidebarWidth = Math.max(
        280,
        Math.min(600, window.innerWidth - e.clientX),
      );
      await tick();
      window.requestAnimationFrame(() => {
        if (monacoEditor) monacoEditor.layout();
        resetEditorScroll();
      });
    }
    if (isDraggingBottom) {
      bottomDrawerHeight = Math.max(
        120,
        Math.min(500, window.innerHeight - e.clientY),
      );
      await tick();
      window.requestAnimationFrame(() => {
        if (monacoEditor) monacoEditor.layout();
        resetEditorScroll();
      });
    }
  }

  function resetEditorScroll() {
    const frame = document.querySelector(".monaco-editor-frame");
    const wrapper = document.querySelector(".monaco-editor-wrapper");
    const container = document.querySelector(".editor-container");
    if (frame) frame.scrollTop = 0;
    if (wrapper) wrapper.scrollTop = 0;
    if (container) container.scrollTop = 0;
  }

  function handleMouseUp() {
    isDraggingLeft = false;
    isDraggingRight = false;
    isDraggingBottom = false;
    document.body.classList.remove('dragging-row', 'dragging-col');
    resetEditorScroll();
  }

  // Draw plot canvas reactively
  $: plotData = $workspaceStore.plotData;
  $: activeBottomTab = $workspaceStore.activeBottomTab;
  $: if (canvasEl && plotData && activeBottomTab === "plotter") {
    setTimeout(drawCanvas, 0);
  }

  onMount(() => {
    actions.loadProjects();
  });

  // Synchronize Monaco editor contents with active file changes
  $: activeFile = $workspaceStore.activeFile;
  $: if (monacoEditor && activeFile) {
    const content = $workspaceStore.fileContents[activeFile] || "";
    if (monacoEditor.getValue() !== content) {
      monacoEditor.setValue(content);
      const isC = activeFile.endsWith(".c") || activeFile.endsWith(".h");
      monaco.editor.setModelLanguage(
        monacoEditor.getModel()!,
        isC ? "c" : "javascript",
      );
    }
  }

  // Auto-scroll terminal output
  $: if ($workspaceStore.serialLogs && terminalEndRef) {
    setTimeout(() => {
      terminalEndRef.scrollIntoView({ behavior: "smooth" });
    }, 50);
  }

  function initMonaco(node: HTMLElement) {
    monacoEditor = monaco.editor.create(node, {
      value: $workspaceStore.fileContents[$workspaceStore.activeFile || ""] || "",
      language: "c",
      theme: "vs-dark",
      automaticLayout: true,
      fontFamily: "JetBrains Mono",
      fontSize: 13,
      minimap: { enabled: false }
    });

    const disposable = monacoEditor.onDidChangeModelContent(() => {
      if ($workspaceStore.activeFile && monacoEditor) {
        actions.updateFileContent($workspaceStore.activeFile, monacoEditor.getValue());
      }
    });

    return {
      destroy() {
        disposable.dispose();
        if (monacoEditor) {
          monacoEditor.dispose();
          monacoEditor = null;
        }
      }
    };
  }

  function drawCanvas() {
    if (!canvasEl) return;
    const ctx = canvasEl.getContext("2d");
    if (!ctx) return;

    const width = canvasEl.clientWidth;
    const height = canvasEl.clientHeight;
    canvasEl.width = width;
    canvasEl.height = height;

    ctx.clearRect(0, 0, width, height);

    // Background Grid
    ctx.strokeStyle = "#12121A";
    ctx.lineWidth = 1;
    for (let i = 40; i < width; i += 60) {
      ctx.beginPath();
      ctx.moveTo(i, 0);
      ctx.lineTo(i, height - 20);
      ctx.stroke();
    }
    for (let i = 20; i < height - 20; i += 30) {
      ctx.beginPath();
      ctx.moveTo(40, i);
      ctx.lineTo(width, i);
      ctx.stroke();
    }

    if ($workspaceStore.plotData.length < 2) {
      ctx.fillStyle = "#64748B";
      ctx.font = "11px Outfit";
      ctx.fillText(
        "Waiting for serial stream telemetry...",
        width / 2 - 100,
        height / 2,
      );
      return;
    }

    const paddingLeft = 40;
    const paddingBottom = 20;
    const graphWidth = width - paddingLeft - 20;
    const graphHeight = height - paddingBottom - 10;

    const temps = $workspaceStore.plotData.map((d) => d.temp);
    const minTemp = Math.min(...temps) - 1;
    const maxTemp = Math.max(...temps) + 1;
    const tempRange = maxTemp - minTemp || 1;

    // Drawing Gradient Line
    ctx.strokeStyle = "#8B5CF6";
    ctx.lineWidth = 2;
    ctx.beginPath();

    $workspaceStore.plotData.forEach((pt, index) => {
      const x = paddingLeft + (index / ($workspaceStore.plotData.length - 1)) * graphWidth;
      const y =
        height -
        paddingBottom -
        ((pt.temp - minTemp) / tempRange) * graphHeight;
      if (index === 0) {
        ctx.moveTo(x, y);
      } else {
        ctx.lineTo(x, y);
      }
    });
    ctx.stroke();

    // Axis
    ctx.strokeStyle = "#334155";
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(paddingLeft, 5);
    ctx.lineTo(paddingLeft, height - paddingBottom);
    ctx.lineTo(width - 10, height - paddingBottom);
    ctx.stroke();

    // Ticks text
    ctx.fillStyle = "#94A3B8";
    ctx.font = "9px JetBrains Mono";
    ctx.fillText(`${maxTemp.toFixed(1)}°C`, 5, 12);
    ctx.fillText(`${minTemp.toFixed(1)}°C`, 5, height - paddingBottom - 4);
  }

  // Compiler / Flash handlers
  function handleBuild() {
    if ($workspaceStore.isCompiling) return;
    actions.setCompiling(true);
    actions.clearBuildLogs();
    actions.addBuildLog("HARDCOREAI Build Engine v1.0.0");
    actions.addBuildLog("Scanning active target configurations...");
    actions.addBuildLog(
      `Found toolchain compiler: ${$workspaceStore.toolchainPath}`,
    );
    actions.addBuildLog(
      `Target architecture: ${$workspaceStore.selectedBoard === "STM32F401" ? "Cortex-M4" : "Xtensa LX7"}`,
    );

    setTimeout(() => {
      actions.addBuildLog("Compiling Core/Src/main.c...");
      actions.addBuildLog("Compiling Core/Src/stm32f4xx_it.c...");
    }, 400);

    setTimeout(() => {
      actions.addBuildLog("Linking build/hardcoreai_app.elf...");
      actions.addBuildLog("──────────────────────────────────────────");
      actions.addBuildLog("Static Memory Utilization statistics:");
      actions.addBuildLog("  FLASH:  26.4 KB / 256.0 KB (10.3%)");
      actions.addBuildLog("  SRAM:   12.1 KB /  64.0 KB (18.9%)");
      actions.addBuildLog("──────────────────────────────────────────");
      actions.addBuildLog(
        "Build Successful. Object binary generated: build/hardcoreai_app.bin",
      );
      actions.setCompiling(false);
    }, 1500);
  }

  function handleFlash() {
    if ($workspaceStore.isFlashing) return;
    actions.setFlashing(true);
    actions.addBuildLog("Launching flashing target engine...");
    actions.addBuildLog(
      `Flashing target via probe: ${$workspaceStore.selectedProbe}`,
    );

    setTimeout(() => {
      actions.addBuildLog("Connection verified. Halting target core...");
      actions.addBuildLog("Erasing sectors... OK");
      actions.addBuildLog("Writing binary image to flash block 0x08000000...");
    }, 400);

    setTimeout(() => {
      actions.addBuildLog("Verifying integrity checksum... OK");
      actions.addBuildLog("Resetting target CPU core. Start execution...");
      actions.setFlashing(false);
      actions.addSerialLog(
        "[SYSTEM] Board reset. Flashed firmware execution initialized.",
      );
    }, 1200);
  }

  function handleDebugToggle() {
    if ($workspaceStore.isDebugging) {
      actions.stopDebugging();
      actions.addBuildLog("Debugger disconnected.");
    } else {
      actions.addBuildLog("Launching GDB debug server...");
      actions.addBuildLog(
        `Probe: ${$workspaceStore.selectedProbe} connected to target: ${$workspaceStore.selectedBoard}`,
      );
      setTimeout(() => {
        actions.startDebugging();
        actions.addBuildLog(
          "Debugger successfully attached. Target halted at main() -> main.c:22",
        );
      }, 800);
    }
  }

  function handleAiSend(e: Event) {
    e.preventDefault();
    if (!aiInput.trim()) return;
    actions.sendAiMessage(aiInput);
    aiInput = "";
  }

  function handleSerialSend(e: Event) {
    e.preventDefault();
    if (!serialInput.trim()) return;
    actions.addSerialLog(`[TX] ${serialInput}`);
    serialInput = "";
  }

  function renderFileNode(item: FileItem) {
    const isFolder = item.isFolder;
    const isActive = $workspaceStore.activeFile === item.path;

    if (isFolder) {
      return {
        isFolder: true,
        item,
        children: item.children || [],
      };
    } else {
      return {
        isFolder: false,
        item,
        isActive,
      };
    }
  }
</script>

<svelte:window onmousemove={handleMouseMove} onmouseup={handleMouseUp} />
<div class="helix-app">
  <!-- 1. Header Command Bar -->
  <header class="helix-header">
    <div class="logo-section">
      <div class="logo-text">HARDCORE<span>AI</span></div>
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div
        class="target-dropdown-pill"
        onclick={() => (showConfigurator = !showConfigurator)}
      >
        <span>Target: {$workspaceStore.selectedBoard}RETx</span>
        <ChevronDown size={11} class="target-dropdown-arrow" />
      </div>
    </div>

    <!-- Center Actions Capsule -->
    <div class="command-capsule">
      <button
        class="capsule-btn build"
        onclick={handleBuild}
        disabled={$workspaceStore.isCompiling || $workspaceStore.isFlashing}
        title="Compile Project"
      >
        <Play size={12} class="play-triangle-fill" />
        <span>{$workspaceStore.isCompiling ? "Compiling..." : "Build"}</span>
      </button>

      <div class="divider-line"></div>

      <button
        class="capsule-btn flash"
        onclick={handleFlash}
        disabled={$workspaceStore.isCompiling || $workspaceStore.isFlashing}
        title="Flash to Device"
      >
        <Zap size={12} />
        <span>{$workspaceStore.isFlashing ? "Flashing..." : "Flash"}</span>
      </button>

      <div class="divider-line"></div>

      <button
        class="capsule-btn debug {$workspaceStore.isDebugging
          ? $workspaceStore.crashed
            ? 'active crashed'
            : 'active debug-running'
          : ''}"
        onclick={handleDebugToggle}
        title="Toggle Debugger (OpenOCD + GDB)"
      >
        <Bug size={12} />
        <span
          >{$workspaceStore.isDebugging
            ? $workspaceStore.crashed
              ? "CRASHED"
              : "Debug"
            : "Debug"}</span
        >
      </button>

      <div class="divider-line"></div>

      <div class="capsule-more-options">
        <MoreHorizontal
          size={13}
          style="color: var(--text-muted); cursor: pointer; padding: 0 4px;"
        />
      </div>
    </div>

    <!-- Connectivity Status & Controls -->
    <div class="connection-status-group">
      <div class="connection-status">
        {#if $workspaceStore.isDebugging && !$workspaceStore.crashed}
          <div
            class="command-capsule"
            style="background: rgba(6, 182, 212, 0.08); border-color: rgba(6, 182, 212, 0.3);"
          >
            <button
              class="capsule-btn"
              style="color: var(--accent-cyan); padding: 4px 8px;"
              onclick={actions.stepOver}
            >
              <span>Step Over</span>
            </button>
            <div
              class="divider-line"
              style="background-color: rgba(6, 182, 212, 0.3);"
            ></div>
            <button
              class="capsule-btn"
              style="color: var(--accent-cyan); padding: 4px 8px;"
              onclick={actions.continueExecution}
            >
              <span>Run</span>
            </button>
          </div>
        {/if}

        {#if !$workspaceStore.crashed}
          <button
            class="status-pill"
            onclick={actions.triggerCrash}
            style="border-color: rgba(239, 68, 68, 0.2); cursor: pointer;"
            title="Trigger Heat Loop Overheat Exception"
          >
            <span class="status-dot" style="background-color: #EF4444;"></span>
            <span style="color: #EF4444;">Simulate Overheat</span>
          </button>
        {:else}
          <button
            class="status-pill"
            onclick={actions.resolveCrash}
            style="border-color: rgba(16, 185, 129, 0.4); cursor: pointer;"
            title="Apply Code Patch Fix"
          >
            <span class="status-dot active"></span>
            <span style="color: #10B981;">Apply AI Patch</span>
          </button>
        {/if}

        <button
          class="status-pill"
          onclick={actions.toggleSerialConnection}
          style="cursor: pointer;"
          title="Toggle UART Serial Port Connection"
        >
          <span
            class="status-dot {$workspaceStore.serialConnected ? 'active' : ''}"
          ></span>
          <span
            >{$workspaceStore.serialConnected
              ? `UART COM4: ${$workspaceStore.baudRate}`
              : "UART Offline"}</span
          >
        </button>

        <button
          class="status-pill"
          onclick={() => actions.setActiveSidebarTab("rag")}
          style="cursor: pointer;"
          title="Active Vector Database Files"
        >
          <span class="status-dot ai-active"></span>
          <span>RAG Active: {$workspaceStore.ragDocuments.length} Docs</span>
        </button>
      </div>

      <!-- Quick Access Right -->
      <div class="tauri-controls-group">
        <Search
          size={14}
          class="control-icon-btn"
          onclick={() => actions.setActiveSidebarTab("search")}
        />
        <Settings
          size={14}
          class="control-icon-btn"
          onclick={() => (showConfigurator = !showConfigurator)}
        />
        <Moon size={14} class="control-icon-btn" />
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div
          class="control-icon-btn close-btn-highlight"
          onclick={() => actions.setShowWelcomeScreen(true)}
        >
          <X size={14} />
        </div>
      </div>
    </div>
  </header>

  {#if $workspaceStore.showWelcomeScreen}
    <div class="welcome-screen">
      <div class="welcome-container">
        <div class="welcome-header">
          <h1 class="welcome-title">HARDCORE<span>AI</span></h1>
          <p class="welcome-subtitle">
            A premium, modern embedded developer workspace. Optimize your
            compilation, flashing, and debug loops directly on target
            microcontrollers with zero unnecessary visual noise.
          </p>
        </div>

        <div class="welcome-grid">
          <div class="welcome-column">
            <h3 class="welcome-section-title">Start</h3>
            <div class="welcome-action-list">
              {#if $workspaceStore.activeProjectId}
                <button
                  class="welcome-action-btn"
                  style="border-color: rgba(6, 182, 212, 0.5); background: rgba(6, 182, 212, 0.05);"
                  onclick={() => actions.setShowWelcomeScreen(false)}
                >
                  <MonitorPlay size={16} class="welcome-action-icon" style="color: var(--accent-cyan);" />
                  <span style="color: var(--accent-cyan); font-weight: 500;">Return to Active Workspace &rarr;</span>
                </button>
              {/if}
              <button
                class="welcome-action-btn"
                onclick={async () => {
                  if ($workspaceStore.projectsList.length > 0) {
                    await actions.loadProject($workspaceStore.projectsList[0].id);
                    actions.setShowWelcomeScreen(false);
                    actions.setActiveSidebarTab("explorer");
                  } else {
                    alert("No recent projects found. Please create one.");
                  }
                }}
              >
                <FolderOpen size={16} class="welcome-action-icon" />
                <span>Open Project Folder...</span>
              </button>
              <button
                class="welcome-action-btn"
                onclick={async () => {
                  if ($workspaceStore.projectsList.length > 0) {
                    await actions.loadProject($workspaceStore.projectsList[0].id);
                    actions.setShowWelcomeScreen(false);
                    actions.setActiveSidebarTab("boards");
                  } else {
                    alert("No recent projects found. Please create one.");
                  }
                }}
              >
                <Settings size={16} class="welcome-action-icon" />
                <span>Configure Target Hardware...</span>
              </button>
              <div class="create-project-row" style="display: flex; gap: 8px; margin-top: 8px;">
                <input 
                  type="text" 
                  id="newProjectName" 
                  placeholder="New Project Name..." 
                  class="welcome-input"
                  style="flex: 1; padding: 12px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.1); border-radius: 6px; color: white; font-family: inherit;"
                />
                <button
                  class="welcome-action-btn"
                  style="width: auto; padding: 0 20px; margin: 0;"
                  onclick={async () => {
                    const inputEl = document.getElementById("newProjectName") as HTMLInputElement;
                    const projectName = inputEl?.value?.trim() || "My Embedded Project";
                    try {
                      const project = await api.createProject(projectName, "Created from IDE");
                      await actions.loadProject(project.id);
                      await actions.loadProjects(); // Refresh the list
                      actions.setActiveSidebarTab("explorer");
                      actions.setShowWelcomeScreen(false);
                      actions.addBuildLog("Created new embedded project template successfully.");
                    } catch (e: any) {
                      actions.addBuildLog("Failed to create project: " + e.message);
                    }
                  }}
                >
                  <Plus size={16} class="welcome-action-icon" />
                  <span>Create</span>
                </button>
              </div>
            </div>
          </div>

          <div class="welcome-column">
            <h3 class="welcome-section-title">Recent Workspaces</h3>
            <div class="recent-list">
              {#each $workspaceStore.projectsList as project}
                <div style="display: flex; align-items: center; justify-content: space-between; padding-right: 12px; gap: 8px; border: 1px solid rgba(255,255,255,0.05); border-radius: 6px; margin-bottom: 8px; background: rgba(0,0,0,0.2);">
                  <div
                    class="recent-item"
                    style="flex: 1; border: none; margin-bottom: 0; background: transparent;"
                    onclick={async () => {
                      await actions.loadProject(project.id);
                      actions.setSelectedBoard("STM32F401");
                      actions.setSelectedProbe("ST-Link V2");
                      actions.setShowWelcomeScreen(false);
                    }}
                  >
                    <div class="recent-name">{project.name}</div>
                    <div class="recent-path">Project ID: {project.id} | {new Date(project.created_at).toLocaleDateString()}</div>
                  </div>
                  <button 
                    class="control-icon-btn close-btn-highlight" 
                    title="Delete Project"
                    style="padding: 6px; border-radius: 4px;"
                    onclick={async (e) => {
                      e.stopPropagation();
                      if (confirm(`Are you sure you want to delete '${project.name}'?`)) {
                        await actions.deleteProject(project.id);
                      }
                    }}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              {/each}
              {#if $workspaceStore.projectsList.length === 0}
                <div class="recent-item" style="opacity: 0.5;">
                  <div class="recent-name">No projects found</div>
                  <div class="recent-path">Create a new template to get started</div>
                </div>
              {/if}
            </div>
          </div>
        </div>

        <div class="welcome-footer">
          <div class="welcome-footer-logo">
            HARDCOREAI v1.0.0 (Renderer: Svelte 5)
          </div>
          <button
            class="welcome-enter-btn"
            onclick={async () => {
              if (!$workspaceStore.activeProjectId) {
                if ($workspaceStore.projectsList.length > 0) {
                  await actions.loadProject($workspaceStore.projectsList[0].id);
                } else {
                  await api.createProject("My Embedded Project", "Created from IDE");
                  await actions.loadProjects();
                  const newProj = $workspaceStore.projectsList[0];
                  if (newProj) await actions.loadProject(newProj.id);
                }
              }
              actions.setShowWelcomeScreen(false);
            }}
          >
            <span>{$workspaceStore.activeProjectId ? 'Return to Workspace' : 'Open Workspace'}</span>
            <ArrowRight size={14} />
          </button>
        </div>
      </div>
    </div>
  {:else}
    <!-- 2. Main Workspace Layout -->
    <div class="helix-main-workspace {$workspaceStore.isDebugging ? 'debug-active' : ''} {aiOpen ? 'ai-open' : ''}">

      <!-- Leftmost Activity Bar -->
      <nav class="activity-bar">
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
      <button
        class="activity-item {$workspaceStore.activeSidebarTab === 'explorer'
          ? 'active'
          : ''}"
        onclick={() => actions.setActiveSidebarTab("explorer")}
        title="Explorer"
      >
        <Folder size={18} />
      </button>
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
      <button
        class="activity-item {$workspaceStore.activeSidebarTab === 'search'
          ? 'active'
          : ''}"
        onclick={() => actions.setActiveSidebarTab("search")}
        title="Search"
      >
        <Search size={18} />
      </button>
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
      <button
        class="activity-item {$workspaceStore.activeSidebarTab === 'git'
          ? 'active'
          : ''}"
        onclick={() => actions.setActiveSidebarTab("git")}
        title="Source Control"
      >
        <GitBranch size={18} />
      </button>
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
      <button
        class="activity-item {$workspaceStore.activeSidebarTab === 'debug'
          ? 'active'
          : ''}"
        onclick={() => actions.setActiveSidebarTab("debug")}
        title="Run & Debug"
      >
        <Bug size={18} />
      </button>
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
      <button
        class="activity-item {$workspaceStore.activeSidebarTab === 'rag'
          ? 'active'
          : ''}"
        onclick={() => actions.setActiveSidebarTab("rag")}
        title="RAG Knowledge Docs"
      >
        <Database size={18} />
      </button>
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
      <button
        class="activity-item {$workspaceStore.activeSidebarTab === 'boards'
          ? 'active'
          : ''}"
        onclick={() => actions.setActiveSidebarTab("boards")}
        title="Target Config"
      >
        <Settings size={18} />
      </button>
    </nav>

    <!-- Sidebar Panel Column -->
    <aside class="workspace-panel sidebar-panel" style="width: {sidebarWidth}px;">
      {#if $workspaceStore.activeSidebarTab === "explorer"}
        <div class="panel-header">
          <div class="panel-title">PROJECT EXPLORER</div>
          <div class="pane-header-actions" style="display: flex; gap: 6px;">
            <Plus
              size={13}
              style="cursor: pointer; color: var(--text-muted);"
              onclick={() => {
                const name = prompt("Enter file name (e.g. src/main.c):");
                if (name) actions.createFile(name);
              }}
              title="New File"
            />
            <FolderOpen
              size={12}
              style="cursor: pointer; color: var(--text-muted);"
              onclick={() => {
                const name = prompt("Enter folder name:");
                if (name) actions.createFolder(name);
              }}
              title="New Folder"
            />
          </div>
        </div>

        <div
          class="panel-body flex-container-explorer"
          style="display: flex; flex-direction: column; gap: 16px;"
        >
          <div class="explorer-section">
            <div class="file-list">
              <div style="margin-bottom: 2px;">
                <!-- svelte-ignore a11y-click-events-have-key-events -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div
                  class="file-item folder"
                  onclick={() => (showConfigurator = !showConfigurator)}
                >
                  <Blocks size={14} style="color: var(--accent-violet);" />
                  <span>Embedded Configurator</span>
                </div>
                <div class="folder-contents">
                  {#each $workspaceStore.fileTree as cat}
                    {@const render = renderFileNode(cat)}
                    {#if render.isFolder}
                      <div style="margin-bottom: 2px;">
                        <div class="file-item folder">
                          <Folder
                            size={14}
                            style="color: var(--accent-violet);"
                          />
                          <span>{render.item.name}</span>
                        </div>
                        <div class="folder-contents">
                          {#each render.children as child}
                            <!-- svelte-ignore a11y-click-events-have-key-events -->
                            <!-- svelte-ignore a11y-no-static-element-interactions -->
                            <div
                              class="file-item {$workspaceStore.activeFile ===
                              child.path
                                ? 'active'
                                : ''}"
                              onclick={() => actions.setActiveFile(child.path)}
                            >
                              <FileCode
                                size={14}
                                style="color: var(--accent-violet-hover);"
                              />
                              <span>{child.name}</span>
                            </div>
                          {/each}
                        </div>
                      </div>
                    {:else}
                      <!-- svelte-ignore a11y-click-events-have-key-events -->
                      <!-- svelte-ignore a11y-no-static-element-interactions -->
                      <div
                        class="file-item {render.isActive ? 'active' : ''}"
                        onclick={() => actions.setActiveFile(render.item.path)}
                      >
                        <File size={14} style="color: var(--text-muted);" />
                        <span>{render.item.name}</span>
                      </div>
                    {/if}
                  {/each}
                </div>
              </div>
            </div>
          </div>

          <!-- RAG Context indicator shortcut inside explorer -->
          <div class="explorer-sub-section">
            <div class="explorer-sub-header">RAG DATABASES CONTEXT</div>
            {#each $workspaceStore.ragDocuments as doc}
              <button
                type="button"
                class="quick-access-item"
                onclick={() => actions.setActiveSidebarTab("rag")}
                style="cursor: pointer; display: flex; align-items: center; justify-content: space-between;"
              >
                <span
                  style="font-family: var(--font-mono); font-size: 0.65rem; color: var(--accent-cyan); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 140px;"
                  >{doc.name}</span
                >
                <span class="shortcut-tag">{doc.size}</span>
              </button>
            {/each}
          </div>

          <div class="explorer-sub-section">
            <div class="explorer-sub-header">EMULATED HARDWARE STATUS</div>
            <div
              class="workspace-item-row"
              style="display: flex; align-items: center; justify-content: space-between; font-size: 0.65rem;"
            >
              <span>Virtual MCU Core:</span>
              <span
                style="color: {$workspaceStore.emulationRunning
                  ? 'var(--accent-success)'
                  : 'var(--text-dark)'}; font-weight: bold;"
              >
                {$workspaceStore.emulationRunning ? "ACTIVE" : "HALTED"}
              </span>
            </div>
          </div>
        </div>
      {/if}

      {#if $workspaceStore.activeSidebarTab === "search"}
        <div class="panel-header">
          <div class="panel-title">Search Workspace</div>
        </div>
        <div class="panel-body">
          <div class="sidebar-search-panel">
            <input type="text" placeholder="Search string..." />
            <input type="text" placeholder="Files to include (e.g. *.c)" />
            <div
              style="font-size: 0.75rem; color: var(--text-dark); margin-top: 10px;"
            >
              No active search results. Press Enter to search.
            </div>
          </div>
        </div>
      {/if}

      {#if $workspaceStore.activeSidebarTab === "git"}
        <div class="panel-header">
          <div class="panel-title">Source Control</div>
        </div>
        <div class="panel-body">
          <div class="sidebar-git-panel">
            <input type="text" placeholder="Commit message (Ctrl+Enter)..." />
            <button class="git-commit-btn">Commit Changes</button>
            <div
              style="font-size: 0.75rem; color: var(--text-muted); margin-top: 12px; border-top: 1px solid var(--border-color); padding-top: 8px;"
            >
              <strong style="display: block; margin-bottom: 4px;"
                >Staged Changes (2)</strong
              >
              <div
                style="padding: 2px 0; color: var(--accent-success); font-family: var(--font-mono); font-size: 0.7rem;"
              >
                M Core/Src/main.c
              </div>
              <div
                style="padding: 2px 0; color: var(--text-muted); font-family: var(--font-mono); font-size: 0.7rem;"
              >
                M CMakeLists.txt
              </div>
            </div>
          </div>
        </div>
      {/if}

      {#if $workspaceStore.activeSidebarTab === "debug"}
        <div class="panel-header">
          <div class="panel-title">Run & Debug GDB</div>
        </div>
        <div class="panel-body">
          <div class="sidebar-debug-panel">
            <div
              style="font-size: 0.75rem; color: var(--text-muted); margin-bottom: 12px;"
            >
              <strong style="display: block; margin-bottom: 4px;"
                >Call Stack</strong
              >
              <div
                style="font-family: var(--font-mono); font-size: 0.7rem; color: var(--text-muted);"
              >
                {$workspaceStore.callStack[0]}
              </div>
              <div
                style="font-family: var(--font-mono); font-size: 0.7rem; color: var(--text-dark);"
              >
                {$workspaceStore.callStack[1]}
              </div>
            </div>
            <div style="font-size: 0.75rem; color: var(--text-muted);">
              <strong style="display: block; margin-bottom: 4px;"
                >Active Breakpoints</strong
              >
              <div
                style="padding: 2px 0; display: flex; align-items: center; gap: 6px;"
              >
                <span
                  style="width: 6px; height: 6px; border-radius: 50%; background-color: var(--accent-error);"
                ></span>
                <span>main.c: Line 24</span>
              </div>
            </div>
          </div>
        </div>
      {/if}

      {#if $workspaceStore.activeSidebarTab === "rag"}
        <RagUploadPanel />
      {/if}

      {#if $workspaceStore.activeSidebarTab === "boards"}
        <div class="panel-header">
          <div class="panel-title">Target Config</div>
        </div>
        <div class="panel-body">
          <div class="boards-config-panel">
            <div class="config-group">
              <!-- svelte-ignore a11y-label-has-associated-control -->
              <label>MCU Board Target</label>
              <select
                class="config-select"
                value={$workspaceStore.selectedBoard}
                onchange={(e) =>
                  actions.setSelectedBoard(e.currentTarget.value as any)}
              >
                <option value="STM32F401">STM32F401 (Cortex-M4)</option>
                <option value="ESP32-S3">ESP32-S3 (Xtensa LX7)</option>
                <option value="RP2040">RP2040 (Cortex-M0+)</option>
              </select>
            </div>
            <div class="config-group">
              <!-- svelte-ignore a11y-label-has-associated-control -->
              <label>Debugger Probe</label>
              <select
                class="config-select"
                value={$workspaceStore.selectedProbe}
                onchange={(e) =>
                  actions.setSelectedProbe(e.currentTarget.value as any)}
              >
                <option value="ST-Link V2">ST-Link V2 (SWD)</option>
                <option value="J-Link">J-Link (SWD/JTAG)</option>
                <option value="CMSIS-DAP">CMSIS-DAP (SWD)</option>
              </select>
            </div>
            <div class="config-group">
              <!-- svelte-ignore a11y-label-has-associated-control -->
              <label>Toolchain compiler Path</label>
              <div class="path-input-wrapper">
                <input
                  type="text"
                  class="config-input"
                  value={$workspaceStore.toolchainPath}
                  onchange={(e) =>
                    actions.setToolchainPath(e.currentTarget.value)}
                />
                <button
                  class="browse-btn"
                  onclick={() =>
                    actions.setToolchainPath("/usr/bin/arm-none-eabi-gcc")}
                  >Reset</button
                >
              </div>
            </div>
          </div>
        </div>
      {/if}
    </aside>

    <!-- Sidebar Drag Handle -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
      class="resize-handle vertical-handle"
      onmousedown={() => { isDraggingLeft = true; document.body.classList.add('dragging-col'); }}
      style="left: {sidebarWidth + 52}px;"
    ></div>

    <!-- Center Workspace Area (Editor + Bottom Drawer) -->
    <main class="center-editor-panel editor-container">
      <!-- Editor Frame -->
      <section class="monaco-editor-frame">
        <!-- Editor Header Tab bar -->
        <div class="editor-tabs">
          {#if $workspaceStore.activeFile}
            <div class="editor-tab active">
              <FileCode size={12} style="color: var(--accent-violet-hover);" />
              <span>{$workspaceStore.activeFile.split("/").pop()}</span>
              <!-- svelte-ignore a11y-click-events-have-key-events -->
              <!-- svelte-ignore a11y-no-static-element-interactions -->
              <span
                class="close-tab"
                onclick={() => actions.setActiveFile(null)}>×</span
              >
            </div>
          {/if}
          <button
            class="configurator-toggle-tab"
            onclick={() => (showConfigurator = !showConfigurator)}
          >
            <Sliders size={11} style="color: var(--text-muted);" />
            <span
              >{showConfigurator
                ? "Switch to Editor"
                : "Embedded Configurator"}</span
            >
          </button>
        </div>

        <!-- Active Editor Display -->
        <div
          class="monaco-editor-wrapper"
          class:hidden={showConfigurator}
        >
          {#if $workspaceStore.activeFile}
            <div class="monaco-container" use:initMonaco></div>
          {:else}
            <div class="empty-editor-state">
              <h2 style="color: var(--text-muted); font-weight: 500; font-size: 1.1rem; letter-spacing: 0.5px; margin-bottom: 2rem;">
                HARDCORE IDE WORKSPACE
              </h2>
              <div class="quick-actions-row">
                <button class="action-card" onclick={() => actions.setActiveSidebarTab("explorer")}>
                  <Folder size={24} style="color: var(--accent-blue);" />
                  <span>Open Project Folder</span>
                </button>
                <button class="action-card" onclick={() => (showConfigurator = true)}>
                  <Settings size={24} style="color: var(--accent-orange);" />
                  <span>Configure Target Hardware</span>
                </button>
                <button class="action-card" onclick={() => (terminalOpen = true)}>
                  <MonitorPlay size={24} style="color: var(--accent-green);" />
                  <span>Open Terminal &rarr;</span>
                </button>
              </div>
            </div>
          {/if}

          {#if $workspaceStore.crashed}
            <div class="crash-overlay">
              <div class="crash-icon-box">
                <AlertTriangle size={24} />
              </div>
              <div class="crash-details">
                <h3>HARDWARE EXCEPTION (Core halted in HardFault_Handler)</h3>
                <p>{$workspaceStore.crashReason}</p>
                <span
                  >Line 45: *crash_trigger = 0xDEADC0DE; (Dereferenced Null
                  Pointer PC: 0x08001A4E)</span
                >
              </div>
              <button class="crash-resolve-btn" onclick={actions.resolveCrash}>
                <Sparkles size={13} />
                Apply AI Hotpatch Fix
              </button>
            </div>
          {/if}
        </div>

        <!-- Configurator view -->
        {#if showConfigurator}
          <div
            class="configurator-container-inner"
            style="height: 100%; width: 100%;"
          >
            <EmbeddedConfigurator
              selectedBoard={$workspaceStore.selectedBoard}
              onClose={() => (showConfigurator = false)}
              isDetached={false}
              onDetach={() => (showConfigurator = false)}
            />
          </div>
        {/if}
      </section>

      <!-- Bottom Drawer Resizer Handle (inline flex child, sits between editor and terminal) -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      {#if terminalOpen}
        <div
          class="resize-handle horizontal-handle"
          onmousedown={() => { isDraggingBottom = true; document.body.classList.add('dragging-row'); }}
        ></div>
      {/if}

      <!-- Bottom Drawer Frame -->
      {#if terminalOpen}
        <footer
          class="helix-bottom-drawer"
          style="height: {bottomDrawerHeight}px;"
        >
          <!-- Tabs bar -->
          <div class="drawer-tabs">
            <div class="tab-group">
              <button
                class="drawer-tab {$workspaceStore.activeBottomTab === 'terminal'
                  ? 'active'
                  : ''}"
                onclick={() => actions.setBottomTab("terminal")}
              >
                <span>SERIAL TERMINAL</span>
              </button>
              <button
                class="drawer-tab {$workspaceStore.activeBottomTab === 'plotter'
                  ? 'active'
                  : ''}"
                onclick={() => actions.setBottomTab("plotter")}
              >
                <span>TELEMETRY PLOTTER</span>
              </button>
              <button
                class="drawer-tab {$workspaceStore.activeBottomTab === 'registers'
                  ? 'active'
                  : ''}"
                onclick={() => actions.setBottomTab("registers")}
              >
                <span>SFR REGISTERS</span>
              </button>
              <button
                class="drawer-tab {$workspaceStore.activeBottomTab === 'emulation'
                  ? 'active'
                  : ''}"
                onclick={() => actions.setBottomTab("emulation")}
              >
                <span>HARDWARE EMULATION</span>
              </button>
              <button
                class="drawer-tab {$workspaceStore.activeBottomTab === 'memory'
                  ? 'active'
                  : ''}"
                onclick={() => actions.setBottomTab("memory")}
              >
                <span>BUILD OUTPUT</span>
              </button>
            </div>
            <div style="margin-left: auto; display: flex; align-items: center; padding-right: 10px;">
              <button class="close-ai-btn" type="button" onclick={() => (terminalOpen = false)} title="Minimize Terminal">
                <X size={13} />
              </button>
            </div>
          </div>

        <!-- Active tab view -->
        <div class="drawer-content">
          {#if $workspaceStore.activeBottomTab === "terminal"}
            <div class="serial-panel">
              <div class="terminal-scroll">
                {#each $workspaceStore.serialLogs as log}
                  <div class="terminal-line">{log}</div>
                {/each}
                <div bind:this={terminalEndRef}></div>
              </div>
              <form class="terminal-input-bar" onsubmit={handleSerialSend}>
                <span class="prompt">COM4 &gt;</span>
                <input
                  type="text"
                  class="terminal-input"
                  placeholder="Send serial bytes to MCU..."
                  bind:value={serialInput}
                />
                <button type="submit">SEND</button>
              </form>
            </div>
          {/if}

          {#if $workspaceStore.activeBottomTab === "plotter"}
            <div class="plotter-panel">
              <div class="plot-stats-overlay">
                <div class="stat-lbl">
                  <span class="stat-dot temp"></span>TEMP:
                  <span class="stat-val"
                    >{$workspaceStore.analogSensors.temp.toFixed(1)} °C</span
                  >
                </div>
                <div class="stat-lbl">
                  <span class="stat-dot volt"></span>VDD:
                  <span class="stat-val"
                    >{$workspaceStore.analogSensors.voltage.toFixed(2)} V</span
                  >
                </div>
                <div class="stat-lbl">
                  <span class="stat-dot curr"></span>IDD:
                  <span class="stat-val"
                    >{$workspaceStore.analogSensors.current.toFixed(1)} mA</span
                  >
                </div>
              </div>
              <div class="plotter-canvas-container">
                <canvas bind:this={canvasEl} class="telemetry-canvas"></canvas>
              </div>
            </div>
          {/if}

          {#if $workspaceStore.activeBottomTab === "registers"}
            <div class="registers-panel">
              <div class="peripheral-list">
                {#each $workspaceStore.registers as reg}
                  <!-- svelte-ignore a11y-click-events-have-key-events -->
                  <!-- svelte-ignore a11y-no-static-element-interactions -->
                  <div
                    class="peripheral-item {selectedPeripheral === reg.name
                      ? 'active'
                      : ''}"
                    onclick={() => (selectedPeripheral = reg.name)}
                  >
                    <div style="display: flex; align-items: center; gap: 8px;">
                      <Cpu size={12} style="color: var(--accent-violet);" />
                      <span>{reg.name}</span>
                    </div>
                    <span class="peripheral-address">{reg.value}</span>
                  </div>
                {/each}
              </div>

              <div class="register-details-grid">
                {#each $workspaceStore.registers as reg}
                  {#if selectedPeripheral === reg.name}
                    {#each reg.bits || [] as bit}
                      <div class="register-row">
                        <div class="register-row-header">
                          <span class="register-name">{bit.name}</span>
                          <span class="register-value">0x{bit.value.toString(16).toUpperCase()}</span>
                        </div>
                        <div class="register-desc">{bit.description}</div>
                        <div style="display: flex; justify-content: space-between; align-items: center; font-size: 0.65rem; color: var(--text-dark); margin-top: 4px;">
                          <span>Range: {bit.range}</span>
                        </div>
                      </div>
                    {/each}
                  {/if}
                {/each}
              </div>
            </div>
          {/if}

          {#if $workspaceStore.activeBottomTab === "emulation"}
            <EmulationPanel />
          {/if}

          {#if $workspaceStore.activeBottomTab === "memory"}
            <div class="serial-panel">
              <div class="terminal-scroll" style="font-family: var(--font-mono);">
                {#each $workspaceStore.buildLogs as log}
                  <div
                    class="terminal-line"
                    style="color: {log.includes('Successful')
                      ? 'var(--accent-success)'
                      : log.includes('Error')
                        ? 'var(--accent-error)'
                        : '#94A3B8'};"
                  >
                    {log}
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </div>
        </footer>
      {/if}

      <!-- Terminal Toggle Pill -->
      {#if !terminalOpen}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="terminal-toggle-pill" onclick={() => (terminalOpen = true)}>
          <Sliders size={12} />
          <span>TERMINAL</span>
        </div>
      {/if}
    </main>

    <!-- Right Panel Resizer Handle -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    {#if aiOpen}
      <div
        class="resize-handle vertical-handle"
        onmousedown={() => { isDraggingRight = true; document.body.classList.add('dragging-col'); }}
        style="right: {rightSidebarWidth}px;"
      ></div>
    {/if}

    <!-- Right AI Panel Column -->
    <aside
      class="split-sidebar-right right-ai-panel"
      style="width: {rightSidebarWidth}px; display: {aiOpen ? 'flex' : 'none'};"
    >
      <!-- Chat Header -->
      <div class="ai-chat-header">
        <div class="ai-chat-header-info">
          <div class="ai-avatar-badge">
            <Sparkles size={12} />
          </div>
          <div>
            <div class="ai-chat-title">HARDCOREAI COPILOT</div>
            <div class="ai-chat-subtitle">Embedded AI Assistant · Online</div>
          </div>
        </div>
        <button class="close-ai-btn" onclick={() => (aiOpen = false)} title="Minimize panel">
          <X size={13} />
        </button>
      </div>

      <!-- Chat messages view -->
      <div class="ai-copilot-chat-content">
        {#each $workspaceStore.aiMessages as msg}
          <div class="chat-row {msg.sender}">
            {#if msg.sender === 'ai'}
              <div class="chat-avatar ai-avatar"><Sparkles size={9} /></div>
            {:else}
              <div class="chat-avatar user-avatar">DEV</div>
            {/if}
            <div class="chat-msg-block {msg.sender}">
              <div class="chat-msg-meta">
                <span class="chat-msg-sender">{msg.sender === 'ai' ? 'HARDCOREAI' : 'You'}</span>
                <span class="chat-msg-time">{msg.timestamp}</span>
              </div>
              <div class="chat-msg-bubble {msg.sender}">
                {#if msg.text.includes('```')}
                  {@const parts = msg.text.split('```')}
                  {#each parts as part, i}
                    {#if i % 2 === 0}
                      {#if part.trim()}<p class="chat-msg-text">{part.trim()}</p>{/if}
                    {:else}
                      {@const codeLines = part.split('\n')}
                      {@const code = codeLines.slice(1).join('\n')}
                      <pre class="chat-code-block"><code>{code}</code></pre>
                    {/if}
                  {/each}
                {:else}
                  <p class="chat-msg-text">{msg.text}</p>
                {/if}

                {#if msg.status === 'waiting_for_user' && msg.options && msg.options.length > 0}
                  <div class="chat-options-container">
                    {#each msg.options as option}
                      <button class="chat-option-btn" onclick={() => actions.sendAiMessage(option)}>
                        {option}
                      </button>
                    {/each}
                  </div>
                {/if}

                {#if msg.status === 'waiting_for_approval'}
                  {#if msg.plan}
                    <div class="chat-plan-preview">
                      <strong>Proposed Plan:</strong>
                      <p>{msg.plan}</p>
                    </div>
                  {/if}
                  <div class="chat-approval-container">
                    <button class="chat-approve-btn" onclick={() => actions.sendAiMessage("APPROVE")}>
                      Accept & Generate
                    </button>
                  </div>
                {/if}
              </div>
            </div>
          </div>
        {/each}

        {#if $workspaceStore.aiWaiting}
          <div class="chat-row ai">
            <div class="chat-avatar ai-avatar"><Sparkles size={9} /></div>
            <div class="chat-msg-block ai">
              <div class="chat-msg-meta">
                <span class="chat-msg-sender">HARDCOREAI</span>
              </div>
              <div class="chat-msg-bubble ai waiting-bubble">
                <span class="dot"></span>
                <span class="dot"></span>
                <span class="dot"></span>
              </div>
            </div>
          </div>
        {/if}
      </div>

      <!-- Input Box -->
      <div class="chat-input-zone">
        <form class="chat-input-form" onsubmit={handleAiSend}>
          <input
            type="text"
            class="chat-input-field"
            placeholder="Ask about registers, RAG docs, or request a code fix..."
            bind:value={aiInput}
          />
          <button type="submit" class="chat-send-btn" disabled={!aiInput.trim()}>
            <Send size={13} />
          </button>
        </form>
        <div class="chat-input-hint">Press Enter to send · Shift+Enter for new line</div>
      </div>
    </aside>

    <!-- AI Panel Collapsed Sidebar Strip -->
    {#if !aiOpen}
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div class="ai-collapsed-strip" onclick={() => (aiOpen = true)} title="Open AI Copilot">
        <div class="ai-collapsed-icon"><Sparkles size={14} /></div>
        <div class="ai-collapsed-label">AI COPILOT</div>
        <div class="ai-collapsed-dot"></div>
      </div>
    {/if}
    </div>
  {/if}
</div>

<style>
  /* Custom layouts specifically needed for Svelte overlays and resize indicators */
  /* Vertical handles (left/right sidebars) remain absolute */
  .vertical-handle {
    position: absolute;
    top: 50px;
    bottom: 0;
    width: 4px;
    cursor: col-resize;
    z-index: 1000;
    transition: background-color 0.2s ease;
  }

  .vertical-handle:hover {
    background-color: var(--accent-violet);
  }

  /* Horizontal handle (bottom terminal) is an inline flex child */
  .horizontal-handle {
    width: 100%;
    height: 4px;
    flex-shrink: 0;
    cursor: row-resize;
    background-color: var(--border-color);
    transition: background-color 0.2s ease;
  }

  .horizontal-handle:hover {
    background-color: var(--accent-violet);
  }

  .crash-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(18, 12, 16, 0.9);
    backdrop-filter: blur(8px);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: 24px;
    z-index: 99;
  }

  .crash-icon-box {
    width: 50px;
    height: 50px;
    border-radius: 50%;
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid var(--accent-error);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--accent-error);
    margin-bottom: 16px;
    box-shadow: 0 0 20px rgba(239, 68, 68, 0.3);
    animation: pulseGlow 1.5s infinite alternate;
  }

  @keyframes pulseGlow {
    from {
      box-shadow: 0 0 10px rgba(239, 68, 68, 0.2);
    }
    to {
      box-shadow: 0 0 25px rgba(239, 68, 68, 0.5);
    }
  }

  .crash-details h3 {
    margin: 0 0 8px 0;
    font-size: 0.95rem;
    color: var(--accent-error);
    font-weight: 700;
    letter-spacing: 0.5px;
  }

  .crash-details p {
    margin: 0 0 8px 0;
    font-size: 0.8rem;
    color: var(--text-bright);
    font-family: var(--font-mono);
  }

  .crash-details span {
    display: block;
    font-size: 0.7rem;
    color: var(--text-muted);
    font-family: var(--font-mono);
    margin-bottom: 20px;
  }

  .crash-resolve-btn {
    background: var(--accent-success);
    border: none;
    border-radius: var(--radius-sm);
    color: white;
    font-size: 0.78rem;
    font-weight: 600;
    padding: 8px 18px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 8px;
    box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3);
    transition: all 0.2s ease;
  }

  .crash-resolve-btn:hover {
    background: var(--accent-success-hover);
    transform: translateY(-1px);
  }

  .configurator-toggle-tab {
    background: none;
    border: none;
    outline: none;
    color: var(--text-muted);
    font-family: var(--font-sans);
    font-size: 0.72rem;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 12px;
    border-left: 1px solid var(--border-color);
    cursor: pointer;
  }

  .configurator-toggle-tab:hover {
    color: var(--text-bright);
    background: #12121a;
  }

  .chat-code-block {
    background: #060609;
    border: 1px solid #1a1a24;
    border-radius: var(--radius-sm);
    padding: 8px;
    font-family: var(--font-mono);
    font-size: 0.68rem;
    color: #f8fafc;
    overflow-x: auto;
    margin: 6px 0 0 0;
  }

  /* Plot statistics */
  .plot-stats-overlay {
    position: absolute;
    top: 10px;
    right: 20px;
    display: flex;
    gap: 12px;
    z-index: 10;
  }

  .stat-lbl {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.68rem;
    font-weight: 600;
    color: var(--text-muted);
    background: rgba(15, 15, 23, 0.85);
    backdrop-filter: blur(4px);
    border: 1px solid var(--border-color);
    border-radius: 4px;
    padding: 4px 8px;
  }

  .stat-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  .stat-dot.temp {
    background: #f59e0b;
  }
  .stat-dot.volt {
    background: #06b6d4;
  }
  .stat-dot.curr {
    background: #10b981;
  }

  .stat-val {
    color: var(--text-bright);
    font-family: var(--font-mono);
  }

  /* Typing indicator dots animation */
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: var(--accent-violet);
    animation: bounce 1.2s infinite ease-in-out;
    display: inline-block;
  }

  .dot:nth-child(2) {
    animation-delay: 0.2s;
  }

  .dot:nth-child(3) {
    animation-delay: 0.4s;
  }

  @keyframes bounce {
    0%, 80%, 100% {
      transform: scale(0.6);
      opacity: 0.5;
    }
    40% {
      transform: scale(1);
      opacity: 1;
    }
  }
  .chat-options-container {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 12px;
  }
  .chat-option-btn {
    background-color: var(--card-bg-light);
    border: 1px solid var(--border-color);
    color: var(--text-color);
    padding: 8px 12px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 13px;
    text-align: left;
    transition: all 0.2s ease;
  }
  .chat-option-btn:hover {
    background-color: var(--primary-light);
    border-color: var(--primary-color);
  }
  .chat-plan-preview {
    background-color: var(--bg-dark);
    padding: 10px;
    border-radius: 4px;
    margin-top: 10px;
    font-size: 13px;
    border-left: 3px solid var(--accent-orange);
  }
  .chat-plan-preview strong {
    color: var(--accent-orange);
    display: block;
    margin-bottom: 4px;
  }
  .chat-plan-preview p {
    margin: 0;
    color: var(--text-color-muted);
  }
  .chat-approval-container {
    margin-top: 12px;
  }
  .chat-approve-btn {
    background-color: var(--accent-orange);
    color: white;
    border: none;
    padding: 8px 16px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 600;
    width: 100%;
    transition: opacity 0.2s ease;
  }
  .chat-approve-btn:hover {
    opacity: 0.9;
  }
</style>
