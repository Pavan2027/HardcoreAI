const BACKEND_URL = import.meta.env.VITE_BACKEND_URL || "http://127.0.0.1:62018";
const EMULATOR_URL = import.meta.env.VITE_EMULATOR_URL || "http://127.0.0.1:62019";
let activeProjectId: string | null = null; // Default to null so Landing Page shows

export const api = {
  setActiveProject(id: string) {
    activeProjectId = id;
  },
  
  // --- Projects API ---
  async getProjects() {
    const res = await fetch(`${BACKEND_URL}/api/projects`, { headers: { "Authorization": "Bearer TEST_TOKEN" } });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  async createProject(name: string, description: string = "") {
    const res = await fetch(`${BACKEND_URL}/api/projects`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Authorization": "Bearer TEST_TOKEN" },
      body: JSON.stringify({ name, description })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  async deleteProject(id: string) {
    const res = await fetch(`${BACKEND_URL}/api/projects/${id}`, {
      method: "DELETE",
      headers: { "Authorization": "Bearer TEST_TOKEN" }
    });
    if (!res.ok) throw new Error(await res.text());
    return true;
  },

  async getProjectFiles(id: string) {
    const res = await fetch(`${BACKEND_URL}/api/projects/${id}/files`, { headers: { "Authorization": "Bearer TEST_TOKEN" } });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  async upsertFile(id: string, path: string, content: string, language: string = "c") {
    // The backend path is a path param, needs URL encoding if it has slashes, though FastAPI path:path handles it
    const res = await fetch(`${BACKEND_URL}/api/projects/${id}/files/${path}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json", "Authorization": "Bearer TEST_TOKEN" },
      body: JSON.stringify({ content, language })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  // --- Backend (Python FastAPI) ---
  
  async uploadRagDocument(file: File) {
    const formData = new FormData();
    formData.append("documents", file);
    const res = await fetch(`${BACKEND_URL}/api/projects/${activeProjectId}/rag/upload`, {
      method: "POST",
      body: formData,
      headers: {
        "Authorization": "Bearer TEST_TOKEN"
      }
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  async listRagDocuments() {
    const res = await fetch(`${BACKEND_URL}/api/projects/${activeProjectId}/rag/documents`, {
      method: "GET",
      headers: { "Authorization": "Bearer TEST_TOKEN" }
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  async deleteRagDocument(filename: string) {
    const res = await fetch(`${BACKEND_URL}/api/projects/${activeProjectId}/rag/documents/${encodeURIComponent(filename)}`, {
      method: "DELETE",
      headers: { "Authorization": "Bearer TEST_TOKEN" }
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  async searchRag(query: string) {
    const res = await fetch(`${BACKEND_URL}/api/projects/${activeProjectId}/rag/search`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer TEST_TOKEN"
      },
      body: JSON.stringify({ query })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  async askAgent(query: string) {
    const res = await fetch(`${BACKEND_URL}/api/projects/${activeProjectId}/agent/solve`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer TEST_TOKEN"
      },
      body: JSON.stringify({ 
        provider: "openrouter",
        problem: query 
      })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  // --- Emulator Service (Go) ---
  
  async buildFirmware(projectId: string) {
    const files = await this.getProjectFiles(projectId);
    const pioFiles = files.map((f: any) => ({
      path: f.path,
      content: f.content
    }));

    if (!pioFiles.find((f: any) => f.path === "platformio.ini")) {
      pioFiles.push({
        path: "platformio.ini",
        content: `[env:disco_f100rb]\nplatform = ststm32\nboard = disco_f100rb\nframework = stm32cube\n`
      });
    }

    const res = await fetch(`${EMULATOR_URL}/platformio/build`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ projectPath: "./Blinky", files: pioFiles })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.text();
  },

  async runEmulation() {
    const res = await fetch(`${EMULATOR_URL}/qemu/run`, {
      method: "POST"
    });
    if (!res.ok) throw new Error(await res.text());
    return res.text();
  },

  streamEmulationLogs(onMessage: (msg: string) => void): EventSource {
    const es = new EventSource(`${EMULATOR_URL}/qemu/stream`);
    es.onmessage = (event) => {
      onMessage(event.data);
    };
    return es;
  },

  async connectDebugger() {
    const res = await fetch(`${EMULATOR_URL}/debug/connect`, { method: "POST" });
    if (!res.ok) throw new Error(await res.text());
    return res.text();
  },

  async getRegisters() {
    const res = await fetch(`${EMULATOR_URL}/debug/registers`, { method: "GET" });
    if (!res.ok) throw new Error(await res.text());
    return res.text();
  },

  async stepDebugger() {
    const res = await fetch(`${EMULATOR_URL}/debug/step`, { method: "POST" });
    if (!res.ok) throw new Error(await res.text());
    return res.text();
  }
};
