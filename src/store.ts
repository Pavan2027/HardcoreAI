import { writable } from "svelte/store";
import { api } from "./api";

export interface FileItem {
  name: string;
  path: string;
  isFolder: boolean;
  children?: FileItem[];
}

export interface RegisterItem {
  name: string;
  value: string;
  description: string;
  bits?: { name: string; value: number; range: string; description: string }[];
}

export interface ChatMessage {
  id: string;
  sender: "user" | "ai";
  text: string;
  timestamp: string;
  status?: string;
  plan?: string;
  options?: string[];
  phase?: string;
  inputType?: "radio" | "checkbox" | "select" | "buttons" | "text";
  selectedValue?: string | string[];
  submitted?: boolean;
}

export interface PlotDataPoint {
  time: string;
  temp: number;
  voltage: number;
  current: number;
}

export interface PinConfig {
  pin: string;
  signal: string;
  mode: string;
  speed: string;
  pull: string;
  label: string;
  af: string;
  enabled: boolean;
}

export interface RagDocument {
  id: string;
  name: string;
  size: string;
  chunks: number;
  status: "Uploading..." | "Chunking..." | "Embedding..." | "Ready in Database";
  tokens: number;
}


// Pin configuration data
const initialPins: PinConfig[] = [];

// RAG Documents initial mock list
const initialRagDocs: RagDocument[] = [];

export const workspaceStore = writable({
  // Project & Files
  activeProjectId: null as string | null,
  projectsList: [] as any[],
  activeFile: null as string | null,
  fileContents: {} as Record<string, string>,
  fileTree: [] as FileItem[],

  // Compilation & Flashing
  isCompiling: false,
  isFlashing: false,
  buildLogs: [] as string[],

  // GDB Debugging
  isDebugging: false,
  debuggerActive: false,
  currentLine: null as number | null,
  breakpoints: [] as number[],
  callStack: [] as string[],
  registers: [] as RegisterItem[],
  crashed: false,
  crashReason: null as string | null,

  // Telemetry & Serial
  serialLogs: [] as string[],
  serialConnected: false,
  activePort: "COM4 (ST-Link Virtual Port)",
  baudRate: 115200,
  plotData: [] as PlotDataPoint[],

  // AI Panel
  aiMessages: [] as ChatMessage[],
  aiWaiting: false,

  // UI Tabs
  activeBottomTab: "terminal" as "terminal" | "plotter" | "registers" | "memory" | "emulation",
  showWelcomeScreen: true,
  activeSidebarTab: "explorer" as "explorer" | "search" | "git" | "debug" | "extensions" | "boards" | "rag",
  selectedBoard: "STM32F401" as "STM32F401" | "ESP32-S3" | "RP2040",
  selectedProbe: "ST-Link V2" as "ST-Link V2" | "J-Link" | "CMSIS-DAP",
  toolchainPath: "/usr/bin/arm-none-eabi-gcc",

  // ── NEW FEATURE STATE ──
  // Interactive Pin Configuration
  pins: initialPins as PinConfig[],
  
  // Emulation State
  emulationRunning: false,
  emulationFrequency: "84 MHz" as "1 Hz" | "10 Hz" | "100 Hz" | "1 kHz" | "10 kHz" | "1 MHz" | "84 MHz",
  emulationLogs: [] as string[],
  analogSensors: {
    temp: 24.5,
    voltage: 3.3,
    current: 42.1
  },

  // RAG Document State
  ragDocuments: initialRagDocs as RagDocument[],
  ragUploadProgress: null as string | null,
  semanticQuery: "",
  semanticResults: [] as { file: string; match: string; score: number }[],
});

// Helper Actions for Store
export const actions = {
  loadProjects: async () => {
    try {
      const projects = await api.getProjects();
      workspaceStore.update(s => ({ ...s, projectsList: projects }));
    } catch (e) {
      console.error("Failed to load projects", e);
    }
  },

  deleteProject: async (id: string) => {
    try {
      await api.deleteProject(id);
      await actions.loadProjects();
    } catch (e) {
      console.error("Failed to delete project", e);
      alert("Failed to delete project");
    }
  },

  // Refresh only the file tree/contents without clearing chat or logs.
  // Used after an agent response so the editor shows new code without losing the conversation.
  refreshProjectFiles: async (id: string) => {
    try {
      const files = await api.getProjectFiles(id);

      const fileContents: Record<string, string> = {};
      const fileTree: FileItem[] = [];

      files.forEach((f: any) => {
        const fullPath = "/" + f.path;
        fileContents[fullPath] = f.content;

        const parts = fullPath.split("/").filter(Boolean);
        let currentLevel = fileTree;
        let builtPath = "";

        parts.forEach((part, i) => {
          builtPath += "/" + part;
          const isFolder = i < parts.length - 1;
          let existing = currentLevel.find(item => item.name === part);

          if (!existing) {
            existing = { name: part, path: builtPath, isFolder, ...(isFolder ? { children: [] } : {}) };
            currentLevel.push(existing);
          }

          if (isFolder && existing.children) {
            currentLevel = existing.children;
          }
        });
      });

      workspaceStore.update(s => ({
        ...s,
        fileTree,
        fileContents,
        // Intentionally do NOT touch aiMessages, buildLogs, emulationLogs, serialLogs
      }));
    } catch (e) {
      console.error("Failed to refresh project files", e);
    }
  },

  loadProject: async (id: string) => {
    try {
      api.setActiveProject(id);
      const files = await api.getProjectFiles(id);
      
      const fileContents: Record<string, string> = {};
      const fileTree: FileItem[] = [];
      
      files.forEach((f: any) => {
        const fullPath = "/" + f.path;
        fileContents[fullPath] = f.content;
        
        const parts = fullPath.split("/").filter(Boolean);
        let currentLevel = fileTree;
        let builtPath = "";
        
        parts.forEach((part, i) => {
          builtPath += "/" + part;
          const isFolder = i < parts.length - 1;
          let existing = currentLevel.find(item => item.name === part);
          
          if (!existing) {
            existing = { name: part, path: builtPath, isFolder, ...(isFolder ? { children: [] } : {}) };
            currentLevel.push(existing);
          }
          
          if (isFolder && existing.children) {
            currentLevel = existing.children;
          }
        });
      });

      let history: ChatMessage[] = [];
      try {
        history = await api.getConversationHistory(id);
        if (history.length === 0) {
          history = [
            {
              id: "default-greeting",
              sender: "ai",
              text: "Hello! I am your HARDCOREAI Copilot. I have loaded context for the **STM32F401RET6** target, SVD registers, and your current `CMake` configuration. \n\nHow can I help you write or debug firmware today?",
              timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
            }
          ];
        }
      } catch (err) {
        console.error("Failed to load chat history", err);
      }

      workspaceStore.update(s => ({
        ...s,
        activeProjectId: id,
        fileTree,
        fileContents,
        activeFile: files.length > 0 ? "/" + files[0].path : null,
        // Clear all session-specific state so previous project data doesn't bleed over
        aiMessages: [],
        buildLogs: [],
        emulationLogs: [],
        serialLogs: [],
        emulationRunning: false,
        isDebugging: false,
        debuggerActive: false,
        currentLine: null,
        crashed: false,
        crashReason: null,

      }));
      
      // Also fetch RAG documents for this project
      await actions.fetchRagDocuments();
    } catch (e) {
      console.error("Failed to load project files", e);
    }
  },

  setActiveFile: (path: string | null) => {
    workspaceStore.update(s => ({ ...s, activeFile: path }));
  },
  
  updateFileContent: (path: string, content: string) => {
    let projectId: string | null = null;
    workspaceStore.update(s => {
      projectId = s.activeProjectId;
      return {
        ...s,
        fileContents: { ...s.fileContents, [path]: content }
      };
    });
    
    if (projectId) {
      // @ts-ignore - store timeout on the window to survive store updates
      clearTimeout(window.__saveTimeout);
      // @ts-ignore
      window.__saveTimeout = setTimeout(async () => {
        try {
          // Remove leading slash if present
          const relPath = path.startsWith('/') ? path.substring(1) : path;
          await api.upsertFile(projectId!, relPath, content);
        } catch (e) {
          console.error("Failed to save file to backend:", e);
        }
      }, 800);
    }
  },
  createFile: async (name: string, folderPath: string = "") => {
    const fullPath = folderPath ? `/${folderPath}/${name}` : `/${name}`;
    let projectId: string | null = null;
    workspaceStore.update(s => {
      projectId = s.activeProjectId;
      if (s.fileContents[fullPath] !== undefined) return s; // already exists
      return {
        ...s,
        fileContents: { ...s.fileContents, [fullPath]: "" },
        activeFile: fullPath
      };
    });
    if (projectId) {
      try {
        const relPath = fullPath.startsWith('/') ? fullPath.substring(1) : fullPath;
        await api.upsertFile(projectId, relPath, "");
        // Refresh the file tree
        await actions.loadProjectFiles(projectId);
      } catch (e) {
        console.error("Failed to create file on backend", e);
      }
    }
  },
  
  createFolder: async (name: string, folderPath: string = "") => {
    // We don't have empty folders in this backend structure, 
    // but we can create a dummy file to represent the folder, 
    // e.g., folderName/.gitkeep
    const dummyFile = folderPath ? `/${folderPath}/${name}/.gitkeep` : `/${name}/.gitkeep`;
    let projectId: string | null = null;
    workspaceStore.update(s => { projectId = s.activeProjectId; return s; });
    if (projectId) {
      try {
        const relPath = dummyFile.startsWith('/') ? dummyFile.substring(1) : dummyFile;
        await api.upsertFile(projectId, relPath, "");
        await actions.loadProjectFiles(projectId);
      } catch(e) {
        console.error("Failed to create folder on backend", e);
      }
    }
  },

  setCompiling: (val: boolean) => {
    workspaceStore.update(s => ({ ...s, isCompiling: val }));
  },
  setFlashing: (val: boolean) => {
    workspaceStore.update(s => ({ ...s, isFlashing: val }));
  },
  addBuildLog: (log: string) => {
    workspaceStore.update(s => ({ ...s, buildLogs: [...s.buildLogs, log] }));
  },
  clearBuildLogs: () => {
    workspaceStore.update(s => ({ ...s, buildLogs: [] }));
  },
  toggleBreakpoint: (line: number) => {
    workspaceStore.update(s => ({
      ...s,
      breakpoints: s.breakpoints.includes(line)
        ? s.breakpoints.filter(l => l !== line)
        : [...s.breakpoints, line]
    }));
  },
  startDebugging: async () => {
    workspaceStore.update(s => ({
      ...s,
      isDebugging: true,
      debuggerActive: true,
      currentLine: 20,
      activeBottomTab: "registers"
    }));
    try {
      await api.connectDebugger();
      const regs = await api.getRegisters();
      actions.addBuildLog(`[GDB] Debugger connected. Registers: \n${regs}`);
    } catch (e: any) {
      actions.addBuildLog(`[GDB] Failed to connect: ${e.message}`);
    }
  },
  stopDebugging: () => {
    workspaceStore.update(s => ({
      ...s,
      isDebugging: false,
      debuggerActive: false,
      currentLine: null,
      crashed: false,
      crashReason: null
    }));
  },
  toggleSerialConnection: () => {
    workspaceStore.update(s => ({ ...s, serialConnected: !s.serialConnected }));
  },
  addSerialLog: (log: string) => {
    workspaceStore.update(s => ({ ...s, serialLogs: [...s.serialLogs, log] }));
  },
  addPlotPoint: (pt: PlotDataPoint) => {
    workspaceStore.update(s => ({ ...s, plotData: [...s.plotData, pt] }));
  },
  setBottomTab: (tab: "terminal" | "plotter" | "registers" | "memory" | "emulation") => {
    workspaceStore.update(s => ({ ...s, activeBottomTab: tab }));
  },
  triggerCrash: () => {
    actions.addBuildLog('Crash UI requires backend GDB integration to trigger manually.');
  },
  resolveCrash: () => {
    actions.addBuildLog('Crash UI requires backend GDB integration to resolve manually.');
  },
  stepOver: async () => {
    try {
      await api.stepDebugger();
      const regs = await api.getRegisters();
      actions.addBuildLog(`[GDB] Stepped. Registers: \n${regs}`);
    } catch (e: any) {
      actions.addBuildLog(`[GDB] Step failed: ${e.message}`);
    }
  },
  continueExecution: () => {
    workspaceStore.update(s => ({ ...s, currentLine: null }));
  },
  setShowWelcomeScreen: (val: boolean) => {
    workspaceStore.update(s => ({ ...s, showWelcomeScreen: val }));
  },
  setActiveSidebarTab: (tab: "explorer" | "search" | "git" | "debug" | "extensions" | "boards" | "rag") => {
    workspaceStore.update(s => ({ ...s, activeSidebarTab: tab }));
  },
  setSelectedBoard: (board: "STM32F401" | "ESP32-S3" | "RP2040") => {
    workspaceStore.update(s => ({ ...s, selectedBoard: board }));
  },
  setSelectedProbe: (probe: "ST-Link V2" | "J-Link" | "CMSIS-DAP") => {
    workspaceStore.update(s => ({ ...s, selectedProbe: probe }));
  },
  setToolchainPath: (path: string) => {
    workspaceStore.update(s => ({ ...s, toolchainPath: path }));
  },

  // ── NEW FEATURE ACTIONS ──
  // Interactive Pin Configuration
  updatePinConfig: (pinName: string, updates: Partial<PinConfig>) => {
    workspaceStore.update(s => {
      const updatedPins = s.pins.map(p => {
        if (p.pin === pinName) {
          return { ...p, ...updates };
        }
        return p;
      });

      return {
        ...s,
        pins: updatedPins
      };
    });
  },
  
  // Emulation Actions
  startEmulation: async () => {
    let projectId: string | null = null;
    workspaceStore.update(s => {
      projectId = s.activeProjectId;
      return {
        ...s,
        emulationRunning: true,
        emulationLogs: [
          ...s.emulationLogs,
          `[EMU] [${new Date().toLocaleTimeString()}] Emulation processor core initialized. Running at ${s.emulationFrequency}`,
          `[EMU] [${new Date().toLocaleTimeString()}] Starting pipeline execution...`
        ]
      };
    });

    if (!projectId) {
      actions.addEmulationLog("Error: No active project to emulate.");
      return;
    }

    try {
      actions.addEmulationLog("Building firmware for QEMU...");
      const buildResultStr = await api.buildFirmware(projectId);
      const buildResult = JSON.parse(buildResultStr);
      
      if (buildResult.output) {
        // Split by newline and add each line to buildLogs
        buildResult.output.split('\\n').forEach((line: string) => {
          if (line.trim()) actions.addBuildLog(line);
        });
      }
      
      if (!buildResult.success) {
        throw new Error(buildResult.error || "Compilation failed");
      }
      
      actions.addEmulationLog("Firmware build complete. Starting emulator...");

      await api.runEmulation();
      
      const es = api.streamEmulationLogs((msg) => {
        actions.addSerialLog(msg);
      });
      
      // Store event source if needed to close it later
      (window as any).__emulationStream = es;

    } catch (e: any) {
      actions.addEmulationLog(`Error starting emulator: ${e.message}`);
    }
  },
  stopEmulation: () => {
    if ((window as any).__emulationStream) {
      (window as any).__emulationStream.close();
      (window as any).__emulationStream = null;
    }
    workspaceStore.update(s => ({
      ...s,
      emulationRunning: false,
      emulationLogs: [
        ...s.emulationLogs,
        `[EMU] [${new Date().toLocaleTimeString()}] Emulation processor core halted. Register context preserved.`
      ]
    }));
  },
  changeEmulationFrequency: (freq: "1 Hz" | "10 Hz" | "100 Hz" | "1 kHz" | "10 kHz" | "1 MHz" | "84 MHz") => {
    workspaceStore.update(s => ({
      ...s,
      emulationFrequency: freq,
      emulationLogs: [
        ...s.emulationLogs,
        `[EMU] [${new Date().toLocaleTimeString()}] Frequency scaled to ${freq}. Core clock refitted.`
      ]
    }));
  },
  updateAnalogSensor: (sensor: "temp" | "voltage" | "current", val: number) => {
    workspaceStore.update(s => ({
      ...s,
      analogSensors: {
        ...s.analogSensors,
        [sensor]: val
      }
    }));
  },
  addEmulationLog: (log: string) => {
    workspaceStore.update(s => ({
      ...s,
      emulationLogs: [...s.emulationLogs, `[EMU] [${new Date().toLocaleTimeString()}] ${log}`]
    }));
  },

  // RAG Document Actions
  fetchRagDocuments: async () => {
    try {
      const res = await api.listRagDocuments();
      workspaceStore.update(s => ({
        ...s,
        ragDocuments: res.documents.map((doc: any) => {
          const name = typeof doc === "string" ? doc : doc.name;
          const sizeBytes = typeof doc === "string" ? 0 : (doc.size || 0);
          const sizeKb = sizeBytes > 0 ? (sizeBytes / 1024).toFixed(1) + " KB" : "Unknown";
          return {
            id: name,
            name: name,
            size: sizeKb,
            chunks: 0,
            status: "Ready in Database",
            tokens: 0
          };
        })
      }));
    } catch (e) {
      console.error("Failed to fetch RAG docs", e);
    }
  },

  uploadDocument: async (file: File) => {
    const sizeStr = (file.size / (1024 * 1024)).toFixed(2) + " MB";
    const id = file.name;

    workspaceStore.update(s => ({
      ...s,
      ragUploadProgress: "Uploading file to RAG database...",
      ragDocuments: [
        { id, name: file.name, size: sizeStr, chunks: 0, status: "Uploading...", tokens: 0 },
        ...s.ragDocuments.filter(d => d.id !== id)
      ]
    }));

    try {
      await api.uploadRagDocument(file);
      
      // Poll until the file appears in the data_dir (backend stages it before ingesting).
      // Large PDFs (20-40 MB) take much longer than 30s to ingest — poll up to 2 minutes.
      let found = false;
      for (let i = 0; i < 120; i++) {
        await new Promise(r => setTimeout(r, 1000));
        const res = await api.listRagDocuments();
        // API returns {documents: [{name, size}, ...]} — must compare .name, not the object itself
        if (res.documents.some((d: any) => (typeof d === "string" ? d : d.name) === file.name)) {
          found = true;
          break;
        }
      }

      if (found) {
        // Refresh from the backend so UI shows the real, up-to-date file list
        await actions.fetchRagDocuments();
        workspaceStore.update(s => ({ ...s, ragUploadProgress: null }));
      } else {
        throw new Error("Timeout waiting for file to be ingested.");
      }
    } catch (e: any) {
      workspaceStore.update(s => ({
        ...s,
        ragUploadProgress: null,
        ragDocuments: s.ragDocuments.filter(d => d.id !== id)
      }));
      alert(`Failed to upload document: ${e.message}`);
    }
  },

  deleteRagDocument: async (filename: string) => {
    try {
      await api.deleteRagDocument(filename);
      workspaceStore.update(s => ({
        ...s,
        ragDocuments: s.ragDocuments.filter(d => d.name !== filename)
      }));
    } catch (e) {
      console.error("Failed to delete doc", e);
    }
  },

  searchRag: async (query: string) => {
    workspaceStore.update(s => ({ ...s, semanticQuery: query }));
    if (!query.trim()) {
      workspaceStore.update(s => ({ ...s, semanticResults: [] }));
      return;
    }
    try {
      const res = await api.searchRag(query);
      let results: any[] = [];
      if (Array.isArray(res.context)) {
        results = res.context.map((match: any) => ({
          file: match.source || "Knowledge Base",
          match: match.text || match.content || (typeof match === 'string' ? match : JSON.stringify(match)),
          score: match.score || 1.0
        }));
      } else if (typeof res.context === 'string') {
        results = [{
          file: "Rag Query Result",
          match: res.context,
          score: 1.0
        }];
      }
      workspaceStore.update(s => ({ ...s, semanticResults: results }));
    } catch (e) {
      console.error("Failed to search RAG", e);
      workspaceStore.update(s => ({ ...s, semanticResults: [] }));
    }
  },
  sendAiMessage: async (text: string) => {
    let projectId: string | null = null;
    workspaceStore.update(state => {
      projectId = state.activeProjectId;
      
      // Mark previous AI message as submitted when user submits an answer
      const updatedMessages = [...state.aiMessages];
      const lastAiIndex = updatedMessages.map(m => m.sender === 'ai').lastIndexOf(true);
      if (lastAiIndex !== -1) {
        updatedMessages[lastAiIndex] = {
          ...updatedMessages[lastAiIndex],
          submitted: true
        };
      }

      return {
        ...state,
        aiMessages: [
          ...updatedMessages,
          {
            id: Math.random().toString(),
            sender: "user",
            text,
            timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
          }
        ],
        aiWaiting: true
      };
    });

    if (projectId) {
      let currentMsgs: ChatMessage[] = [];
      workspaceStore.subscribe(s => { currentMsgs = s.aiMessages; })();
      api.saveConversationHistory(projectId, currentMsgs).catch(console.error);
    }
      
    try {
      let history: any[] = [];
      let currentPhase: string | undefined = undefined;
      let selectedProvider = "openrouter";
      workspaceStore.update(s => {
        history = s.aiMessages.map(m => ({
          role: m.sender === "ai" ? "assistant" : "user",
          content: m.text
        }));

        const lastAiMsg = [...s.aiMessages].reverse().find(m => m.sender === "ai" && m.phase);
        if (lastAiMsg) {
          currentPhase = lastAiMsg.phase;
        }

        if (lastAiMsg?.phase) currentPhase = lastAiMsg.phase;
        selectedProvider = (s as any).selectedProvider || "openrouter";
        return s;
      });

      // Simulation command interceptor (for UI testing)
      let response: any;
      if (text.toLowerCase().startsWith("simulate ")) {
        const cmd = text.toLowerCase().substring(9).trim();
        if (cmd === "radio") {
          response = {
            wiring: {
              status: "waiting_for_user",
              question: "Please select the target SPI clock polarity (CPOL):",
              options: ["CPOL = 0 (Clock active high, idle low)", "CPOL = 1 (Clock active low, idle high)"],
              inputType: "radio",
              phase: "spi_setup"
            }
          };
        } else if (cmd === "checkbox") {
          response = {
            wiring: {
              status: "waiting_for_user",
              question: "Select the GPIO peripherals you want to enable:",
              options: ["GPIOA (pins PA0-PA15)", "GPIOB (pins PB0-PB15)", "GPIOC (pins PC0-PC15)", "GPIOD (pins PD0-PD15)"],
              inputType: "checkbox",
              phase: "gpio_setup"
            }
          };
        } else if (cmd === "select") {
          response = {
            wiring: {
              status: "waiting_for_user",
              question: "Choose a prescaler value for the timer clock division:",
              options: ["Prescaler = 1", "Prescaler = 2", "Prescaler = 4", "Prescaler = 8", "Prescaler = 16"],
              inputType: "select",
              phase: "timer_setup"
            }
          };
        } else if (cmd === "approval") {
          response = {
            wiring: {
              status: "waiting_for_approval",
              question: "Do you approve this configuration plan?",
              final: "1. Enable RCC clock for GPIOA.\n2. Configure PA5 mode register (MODER) as output.\n3. Configure speed register (OSPEEDR) as Medium speed.\n4. Initialize state register (ODR) as low.",
              phase: "approval_phase"
            }
          };
        } else {
          response = await api.askAgent(text, history, currentPhase, selectedProvider);
        }
      } else {
        response = await api.askAgent(text, history, currentPhase, selectedProvider);
      }

      
      let currentActiveProjectId: string | null = null;
      workspaceStore.update(s => {
        currentActiveProjectId = s.activeProjectId;
        return s;
      });
      
      if (currentActiveProjectId) {
        // Only refresh files for real agent calls — skip for simulate test commands.
        // Use refreshProjectFiles (not loadProject) so the chat history is preserved.
        if (!text.toLowerCase().startsWith("simulate ")) {
          await actions.refreshProjectFiles(currentActiveProjectId);
        }

      }

      workspaceStore.update(s => {
        // Use the agent's actual final response text (answer, summary, or explanation).
        // Fall back to a generic message only if the agent produced no text at all.
        let aiText = response.coding?.final || response.wiring?.final || "I successfully completed your request.";
        let newStatus = "completed";
        let newPlan = undefined;
        let newOptions: string[] | undefined = undefined;
        let newPhase = undefined;
        let newInputType: any = "buttons";

        const agentResponse = response.wiring || response.coding;
        if (agentResponse) {
          aiText = agentResponse.question || "I need more information.";
          newStatus = agentResponse.status;
          newOptions = agentResponse.options;
          newPhase = agentResponse.phase;
          newInputType = agentResponse.inputType || "buttons";
          if (newStatus === "waiting_for_approval") {
             newPlan = agentResponse.final;
             aiText = "Do you approve this plan?";
          }
        }

        const newMsg: ChatMessage = {
          id: Math.random().toString(),
          sender: "ai",
          text: aiText,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
          status: newStatus,
          plan: newPlan,
          options: newOptions,
          phase: newPhase,
          inputType: newInputType,
          submitted: false
        };

        const updatedMsgs = [...s.aiMessages, newMsg];
        if (projectId) {
          api.saveConversationHistory(projectId, updatedMsgs).catch(console.error);
        }

        return {
          ...s,
          aiMessages: updatedMsgs,
          aiWaiting: false
        };
      });
    } catch (e: any) {
      workspaceStore.update(s => {
        const errorMsg: ChatMessage = {
          id: Math.random().toString(),
          sender: "ai",
          text: `❌ **Error connecting to agent:** ${e.message}`,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        };
        const updatedMsgs = [...s.aiMessages, errorMsg];
        if (projectId) {
          api.saveConversationHistory(projectId, updatedMsgs).catch(console.error);
        }
        return {
          ...s,
          aiMessages: updatedMsgs,
          aiWaiting: false
        };
      });
    }
  },

  clearChat: async (projectId: string) => {
    try {
      await api.deleteConversationHistory(projectId);
      workspaceStore.update(s => ({
        ...s,
        aiMessages: [
          {
            id: "default-greeting",
            sender: "ai",
            text: "Hello! I am your HARDCOREAI Copilot. I have loaded context for the **STM32F401RET6** target, SVD registers, and your current `CMake` configuration. \n\nHow can I help you write or debug firmware today?",
            timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
          }
        ]
      }));
    } catch (e) {
      console.error("Failed to clear chat history", e);
    }
  },

  renameProject: async (id: string, name: string) => {
    try {
      await api.renameProject(id, name);
      await actions.loadProjects();
    } catch (e) {
      console.error("Failed to rename project", e);
      alert("Failed to rename project");
    }
  },

  deleteActiveProject: async (id: string) => {
    try {
      if (confirm(`Are you sure you want to delete the active project? This will permanently erase all project files.`)) {
        await api.deleteProject(id);
        await actions.loadProjects();
        workspaceStore.update(s => ({
          ...s,
          activeProjectId: null,
          showWelcomeScreen: true
        }));
      }
    } catch (e) {
      console.error("Failed to delete project", e);
      alert("Failed to delete project");
    }
  }
};
