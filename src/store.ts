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

      workspaceStore.update(s => ({
        ...s,
        activeProjectId: id,
        fileTree,
        fileContents,
        activeFile: files.length > 0 ? "/" + files[0].path : null
      }));
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
      await api.buildFirmware(projectId);
      actions.addEmulationLog("Firmware build complete. Starting emulator...");

      await api.runEmulation();
      
      const es = api.streamEmulationLogs((msg) => {
        actions.addEmulationLog(msg);
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
        ragDocuments: res.documents.map((name: string) => ({
          id: name,
          name: name,
          size: "Unknown",
          chunks: 0,
          status: "Ready in Database",
          tokens: 0
        }))
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
      
      // Poll until the file shows up in listRagDocuments
      let found = false;
      for (let i = 0; i < 30; i++) {
        await new Promise(r => setTimeout(r, 1000));
        const res = await api.listRagDocuments();
        if (res.documents.includes(file.name)) {
          found = true;
          break;
        }
      }

      if (found) {
        workspaceStore.update(s => ({
          ...s,
          ragUploadProgress: null,
          ragDocuments: s.ragDocuments.map(d => d.id === id ? { ...d, status: "Ready in Database" } : d)
        }));
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
    workspaceStore.update(state => ({
      ...state,
      aiMessages: [
        ...state.aiMessages,
        {
          id: Math.random().toString(),
          sender: "user",
          text,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        }
      ],
      aiWaiting: true
    }));
      
    try {
      let history: any[] = [];
      let currentPhase: string | undefined = undefined;

      workspaceStore.update(s => {
        // Collect everything up to the current message (which was just added as 'user' above)
        history = s.aiMessages.map(m => ({
          role: m.sender === "ai" ? "assistant" : "user",
          content: m.text
        }));

        // Find the last known phase from AI messages
        const lastAiMsg = [...s.aiMessages].reverse().find(m => m.sender === "ai" && m.phase);
        if (lastAiMsg) {
          currentPhase = lastAiMsg.phase;
        }

        return s;
      });

      const response = await api.askAgent(text, history, currentPhase);
      
      let projectId: string | null = null;
      workspaceStore.update(s => {
        projectId = s.activeProjectId;
        return s;
      });
      
      if (projectId) {
        await actions.loadProject(projectId);
      }

      workspaceStore.update(s => {
        let aiText = "I successfully completed your request.";
        let newStatus = "completed";
        let newPlan = undefined;
        let newOptions: string[] | undefined = undefined;
        let newPhase = undefined;

        // Check if agent is waiting for user or approval
        if (response.wiring?.status === "waiting_for_user" || response.wiring?.status === "waiting_for_approval") {
          aiText = response.wiring.question || "I need more information.";
          newStatus = response.wiring.status;
          newOptions = response.wiring.options;
          newPhase = response.wiring.phase;
          if (newStatus === "waiting_for_approval") {
             newPlan = response.wiring.final;
             aiText = "Do you approve this plan?";
          }
        } else if (response.coding?.status === "waiting_for_user" || response.coding?.status === "waiting_for_approval") {
          aiText = response.coding.question || "I need more information.";
          newStatus = response.coding.status;
          newOptions = response.coding.options;
          newPhase = response.coding.phase;
          if (newStatus === "waiting_for_approval") {
             newPlan = response.coding.final;
             aiText = "Do you approve this plan?";
          }
        }

        return {
          ...s,
          aiMessages: [
            ...s.aiMessages,
            {
              id: Math.random().toString(),
              sender: "ai",
              text: aiText,
              timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
              status: newStatus,
              plan: newPlan,
              options: newOptions,
              phase: newPhase
            }
          ],
          aiWaiting: false
        };
      });
    } catch (e: any) {
      workspaceStore.update(s => ({
        ...s,
        aiMessages: [
          ...s.aiMessages,
          {
            id: Math.random().toString(),
            sender: "ai",
            text: `❌ **Error connecting to agent:** ${e.message}`,
            timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
          }
        ],
        aiWaiting: false
      }));
    }
  }
};
