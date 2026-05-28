import { writable } from "svelte/store";

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

const mockFiles: FileItem[] = [
  {
    name: "src",
    path: "/src",
    isFolder: true,
    children: [
      { name: "main.c", path: "/src/main.c", isFolder: false },
      { name: "stm32f4xx_it.c", path: "/src/stm32f4xx_it.c", isFolder: false },
      { name: "system_stm32f4xx.c", path: "/src/system_stm32f4xx.c", isFolder: false }
    ]
  },
  {
    name: "include",
    path: "/include",
    isFolder: true,
    children: [
      { name: "main.h", path: "/include/main.h", isFolder: false },
      { name: "stm32f4xx_it.h", path: "/include/stm32f4xx_it.h", isFolder: false }
    ]
  },
  {
    name: "CMakeLists.txt",
    path: "/CMakeLists.txt",
    isFolder: false
  },
  {
    name: "stm32f401.ld",
    path: "/stm32f401.ld",
    isFolder: false
  }
];

const mockMainC = `// HARDCOREAI: Blinky Firmware for STM32F401RET6
#include "main.h"

/* Private variables ---------------------------------------------------------*/
GPIO_InitTypeDef GPIO_InitStruct = {0};

/* Private function prototypes -----------------------------------------------*/
void SystemClock_Config(void);
static void MX_GPIO_Init(void);

/**
  * @brief  The application entry point.
  * @retval int
  */
// CPU configuration registers, SVD mapping initialized
// target: STM32F401RETx
// debugger: ST-LINK/V2 (SWD interface)
// clocks: HSE osc crystal at 8 MHz
// system frequency: 84 MHz
//

int main(void)
{
  HAL_Init();
  SystemClock_Config();
  MX_GPIO_Init();
  while (1)
  {
    HAL_GPIO_TogglePin(GPIOA, GPIO_PIN_5);
    HAL_Delay(500);
  }
}

/**
 * @brief System Clock Configuration
 * @retval None
 */
void SystemClock_Config(void)
{
  RCC_OscInitTypeDef RCC_OscInitStruct = {0};
  RCC_ClkInitTypeDef RCC_ClkInitStruct = {0};
  
  // Configure the main internal regulator output voltage */
  __HAL_RCC_PWR_CLK_ENABLE();
  __HAL_PWR_VOLTAGESCALING_CONFIG(PWR_REGULATOR_VOLTAGE_SCALE1);
  
  // Initializes the CPU, AHB and APB buses clocks */
  RCC_OscInitStruct.OscillatorType = RCC_OSCILLATORTYPE_HSE;
  RCC_OscInitStruct.HSEState = RCC_HSE_ON;
  RCC_OscInitStruct.PLL.PLLState = RCC_PLL_ON;
  RCC_OscInitStruct.PLL.PLLSource = RCC_PLLSOURCE_HSE;
}`;

const mockItC = `#include "main.h"
#include "stm32f4xx_it.h"

void NMI_Handler(void) {
}

void HardFault_Handler(void) {
  // Capture crash register values
  printf("!!! HARD_FAULT_INTERRUPT !!!\\r\\n");
  while (1) {
  }
}
`;

// Pin configuration data
const initialPins: PinConfig[] = [
  { pin: "PA5", signal: "SPI1_SCK", mode: "Alternate Function", speed: "High", pull: "No pull-up/down", label: "MCU LED Link", af: "AF5", enabled: true },
  { pin: "PA6", signal: "SPI1_MISO", mode: "Alternate Function", speed: "High", pull: "No pull-up/down", label: "Sensor RX Channel", af: "AF5", enabled: true },
  { pin: "PA7", signal: "SPI1_MOSI", mode: "Alternate Function", speed: "High", pull: "No pull-up/down", label: "Sensor TX Channel", af: "AF5", enabled: true },
  { pin: "PA2", signal: "USART2_TX", mode: "Alternate Function", speed: "Medium", pull: "Pull-up", label: "Serial Debug Output", af: "AF7", enabled: true },
  { pin: "PA3", signal: "USART2_RX", mode: "Alternate Function", speed: "Medium", pull: "Pull-up", label: "Serial Command In", af: "AF7", enabled: true },
  { pin: "PB8", signal: "I2C1_SCL", mode: "Alternate Function", speed: "Medium", pull: "Pull-up", label: "EEPROM Clock Line", af: "AF4", enabled: true },
  { pin: "PB9", signal: "I2C1_SDA", mode: "Alternate Function", speed: "Medium", pull: "Pull-up", label: "EEPROM Data Line", af: "AF4", enabled: true },
  { pin: "PC13", signal: "GPIO_Output", mode: "Output Push Pull", speed: "Low", pull: "No pull-up/down", label: "User Push Button Indicator", af: "-", enabled: true },
  { pin: "PA0", signal: "Analog_IN0", mode: "Analog Mode", speed: "Low", pull: "No pull-up/down", label: "ADC Potentiometer Node", af: "-", enabled: false },
  { pin: "PA4", signal: "DAC_OUT1", mode: "Analog Mode", speed: "Low", pull: "No pull-up/down", label: "Analog Sine Wave Gen", af: "-", enabled: false },
  { pin: "PC0", signal: "Unassigned", mode: "Input Floating", speed: "Low", pull: "No pull-up/down", label: "General Pin", af: "-", enabled: false },
  { pin: "PC1", signal: "Unassigned", mode: "Input Floating", speed: "Low", pull: "No pull-up/down", label: "General Pin", af: "-", enabled: false },
];

// RAG Documents initial mock list
const initialRagDocs: RagDocument[] = [
  { id: "1", name: "STM32F401_Reference_Manual.pdf", size: "14.2 MB", chunks: 2450, status: "Ready in Database", tokens: 840200 },
  { id: "2", name: "ILI9341_TFT_Datasheet.pdf", size: "1.8 MB", chunks: 320, status: "Ready in Database", tokens: 98400 },
  { id: "3", name: "main_motor_controller.c", size: "24 KB", chunks: 18, status: "Ready in Database", tokens: 5400 }
];

export const workspaceStore = writable({
  // Project & Files
  activeFile: "/src/main.c" as string | null,
  fileContents: {
    "/src/main.c": mockMainC,
    "/src/stm32f4xx_it.c": mockItC,
    "/CMakeLists.txt": "cmake_minimum_required(VERSION 3.16)\nproject(hardcoreai_app C)\n\nset(CMAKE_C_STANDARD 11)\nadd_executable(hardcoreai_app src/main.c src/stm32f4xx_it.c)",
    "/stm32f401.ld": "/* Linker Script for STM32F401 */\nMEMORY {\n  FLASH (rx) : ORIGIN = 0x08000000, LENGTH = 256K\n  RAM (xrw)  : ORIGIN = 0x20000000, LENGTH = 64K\n}"
  } as Record<string, string>,
  fileTree: mockFiles,

  // Compilation & Flashing
  isCompiling: false,
  isFlashing: false,
  buildLogs: [
    "HARDCOREAI Build Engine v1.0.0",
    "Initializing CMake project configuration...",
    "Found toolchain: arm-none-eabi-gcc 12.3.1",
    "Ready to build project."
  ] as string[],

  // GDB Debugging
  isDebugging: false,
  debuggerActive: false,
  currentLine: null as number | null,
  breakpoints: [24] as number[],
  callStack: ["main() at main.c:20", "Reset_Handler() at startup_stm32f401.s:55"] as string[],
  registers: [
    {
      name: "GPIOA",
      value: "0x40020000",
      description: "General-Purpose I/O Port A",
      bits: [
        { name: "MODER", value: 0x28000280, range: "31:0", description: "GPIO port mode register" },
        { name: "OTYPER", value: 0x00000000, range: "15:0", description: "GPIO port output type register" },
        { name: "ODR", value: 0x00000020, range: "15:0", description: "GPIO port output data register" }
      ]
    },
    {
      name: "ADC1",
      value: "0x40012000",
      description: "Analog-to-Digital Converter 1",
      bits: [
        { name: "SR", value: 0x00000002, range: "5:0", description: "ADC status register" },
        { name: "CR1", value: 0x00000100, range: "25:0", description: "ADC control register 1" },
        { name: "DR", value: 0x00000A23, range: "11:0", description: "ADC regular data register" }
      ]
    },
    {
      name: "Core Registers",
      value: "CPU Core",
      description: "ARM Cortex-M4 Core Registers",
      bits: [
        { name: "R0", value: 0x00000000, range: "32b", description: "Argument / result register" },
        { name: "R1", value: 0x20000400, range: "32b", description: "General purpose register" },
        { name: "PC", value: 0x080010AC, range: "32b", description: "Program Counter" },
        { name: "LR", value: 0x080012A3, range: "32b", description: "Link Register (return address)" },
        { name: "SP", value: 0x2000FFC0, range: "32b", description: "Stack Pointer" }
      ]
    }
  ] as RegisterItem[],
  crashed: false,
  crashReason: null as string | null,

  // Telemetry & Serial
  serialLogs: [
    "[12:41:10.123] System Booting...",
    "[12:41:10.456] MCU: STM32F401RETx",
    "[12:41:10.457] Clock: 84 MHz",
    "[12:41:10.458] Flash: 512 KB | RAM: 96 KB",
    "[12:41:10.459] Hello from HardcoreAI IDE! 🚀"
  ] as string[],
  serialConnected: true,
  activePort: "COM4 (ST-Link Virtual Port)",
  baudRate: 115200,
  plotData: [
    { time: "00:01", temp: 24.5, voltage: 3.3, current: 42.1 },
    { time: "00:02", temp: 25.1, voltage: 3.3, current: 42.2 },
    { time: "00:03", temp: 26.3, voltage: 3.28, current: 44.5 },
    { time: "00:04", temp: 27.2, voltage: 3.29, current: 43.1 }
  ] as PlotDataPoint[],

  // AI Panel
  aiMessages: [
    {
      id: "1",
      sender: "ai",
      text: "Hello! I am your HARDCOREAI Copilot. I have loaded context for the **STM32F401RET6** target, SVD registers, and your current `CMake` configuration. \n\nHow can I help you write or debug firmware today?",
      timestamp: "21:52"
    }
  ] as ChatMessage[],
  aiWaiting: false,

  // UI Tabs
  activeBottomTab: "terminal" as "terminal" | "plotter" | "registers" | "memory" | "emulation",
  showWelcomeScreen: false,
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
  emulationLogs: [
    "[EMU] Boot ROM loaded at 0x00000000",
    "[EMU] Clock configuration initialized HSE -> PLL (84 MHz)",
    "[EMU] Core reset successfully. Halting at Reset_Handler"
  ] as string[],
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
  setActiveFile: (path: string | null) => {
    workspaceStore.update(s => ({ ...s, activeFile: path }));
  },
  updateFileContent: (path: string, content: string) => {
    workspaceStore.update(s => ({
      ...s,
      fileContents: { ...s.fileContents, [path]: content }
    }));
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
  startDebugging: () => {
    workspaceStore.update(s => ({
      ...s,
      isDebugging: true,
      debuggerActive: true,
      currentLine: 20,
      activeBottomTab: "registers"
    }));
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
    workspaceStore.update(s => {
      const crashRegs = s.registers.map(reg => {
        if (reg.name === "Core Registers") {
          return {
            ...reg,
            bits: reg.bits?.map(bit => {
              if (bit.name === "PC") return { ...bit, value: 0x08001A4E };
              if (bit.name === "R0") return { ...bit, value: 0x00000000 };
              return bit;
            })
          };
        }
        return reg;
      });

      return {
        ...s,
        crashed: true,
        crashReason: "HardFault: Precise Data Bus Error (Dereferencing NULL pointer)",
        currentLine: 45,
        registers: crashRegs,
        activeBottomTab: "registers"
      };
    });
  },
  resolveCrash: () => {
    workspaceStore.update(s => {
      const currentMain = s.fileContents["/src/main.c"] || "";
      const fixedMain = currentMain.includes("uint32_t *crash_trigger = NULL;")
        ? currentMain.replace("uint32_t *crash_trigger = NULL;", "static uint32_t val_holder = 0;\n  uint32_t *crash_trigger = &val_holder;")
        : currentMain;
      
      return {
        ...s,
        crashed: false,
        crashReason: null,
        currentLine: 20,
        fileContents: {
          ...s.fileContents,
          "/src/main.c": fixedMain
        }
      };
    });
  },
  stepOver: () => {
    workspaceStore.update(s => {
      if (s.currentLine === null) return s;
      let nextLine = s.currentLine + 1;
      if (nextLine > 50) nextLine = 20;
      return { ...s, currentLine: nextLine };
    });
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
  startEmulation: () => {
    workspaceStore.update(s => ({
      ...s,
      emulationRunning: true,
      emulationLogs: [
        ...s.emulationLogs,
        `[EMU] [${new Date().toLocaleTimeString()}] Emulation processor core initialized. Running at ${s.emulationFrequency}`,
        `[EMU] [${new Date().toLocaleTimeString()}] Starting pipeline execution...`
      ]
    }));
  },
  stopEmulation: () => {
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
  uploadDocument: (name: string, sizeBytes: number) => {
    const sizeStr = (sizeBytes / (1024 * 1024)).toFixed(2) + " MB";
    const id = Math.random().toString();
    const tokenEst = Math.floor(sizeBytes * 0.06);

    workspaceStore.update(s => ({
      ...s,
      ragUploadProgress: "Uploading files to core intelligence database...",
      ragDocuments: [
        { id, name, size: sizeStr, chunks: 0, status: "Uploading...", tokens: 0 },
        ...s.ragDocuments
      ]
    }));

    setTimeout(() => {
      workspaceStore.update(s => ({
        ...s,
        ragUploadProgress: "Parsing text layout and chunking document streams...",
        ragDocuments: s.ragDocuments.map(d => d.id === id ? { ...d, status: "Chunking..." } : d)
      }));
    }, 1500);

    setTimeout(() => {
      workspaceStore.update(s => ({
        ...s,
        ragUploadProgress: "Generating semantic dense vector embeddings...",
        ragDocuments: s.ragDocuments.map(d => d.id === id ? { ...d, status: "Embedding...", chunks: Math.floor(tokenEst / 300) } : d)
      }));
    }, 3000);

    setTimeout(() => {
      workspaceStore.update(s => ({
        ...s,
        ragUploadProgress: null,
        ragDocuments: s.ragDocuments.map(d => d.id === id ? { ...d, status: "Ready in Database", tokens: tokenEst } : d),
        aiMessages: [
          ...s.aiMessages,
          {
            id: Math.random().toString(),
            sender: "ai",
            text: `📚 **Knowledge base updated!** I have parsed and indexed \`${name}\` into the active RAG contextual engine (split into **${Math.floor(tokenEst / 300)} semantic chunks**). You can now ask questions directly relative to this document!`,
            timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
          }
        ]
      }));
    }, 4500);
  },
  searchRag: (query: string) => {
    workspaceStore.update(s => {
      if (!query.trim()) {
        return { ...s, semanticQuery: query, semanticResults: [] };
      }

      // Simulated semantic search results based on query terms
      let results = [
        { file: "STM32F401_Reference_Manual.pdf", match: "Section 8.3: GPIO Port Output Registers (GPIOx_ODR). Bits 15:0 represent the digital output levels of the corresponding I/O pins.", score: 0.92 },
        { file: "STM32F401_Reference_Manual.pdf", match: "Section 10.4: SPI Mode Control. Alternate function mapping AF5 configures PA5 as SPI1_SCK, PA6 as SPI1_MISO, and PA7 as SPI1_MOSI.", score: 0.88 },
        { file: "ILI9341_TFT_Datasheet.pdf", match: "Pin Function: D/CX (Register Select) controls whether the data stream represents display coordinates or command indices.", score: 0.74 }
      ];

      if (query.toLowerCase().includes("usart") || query.toLowerCase().includes("serial")) {
        results = [
          { file: "STM32F401_Reference_Manual.pdf", match: "Section 19.3: USART Baud Rate Generation. The baud rate clock is derived from the system clock divided by a fractional prescaler.", score: 0.95 },
          { file: "STM32F401_Reference_Manual.pdf", match: "USART2 is mapped to alternate function AF7 on pins PA2 (TX) and PA3 (RX). Ensure clock is enabled in RCC_APB1ENR.", score: 0.89 }
        ];
      }

      return {
        ...s,
        semanticQuery: query,
        semanticResults: results
      };
    });
  },
  sendAiMessage: (text: string) => {
    workspaceStore.update(state => {
      const newMsgs = [
        ...state.aiMessages,
        {
          id: Math.random().toString(),
          sender: "user" as const,
          text,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        }
      ];
      
      // Setup response simulation
      setTimeout(() => {
        let aiResponse = "I am scanning your active workspace and RAG databases...";
        
        if (text.toLowerCase().includes("crash") || text.toLowerCase().includes("fault")) {
          aiResponse = "Analyzing crash dump... GDB reports `CFSR = 0x00008200` which corresponds to a **Precise Data Bus Error**. You are attempting to write to address `0x00000000` (NULL pointer) inside `R0` during the execution at PC `0x08001A4E` (*crash_trigger = 0xDEADC0DE).\n\nTo resolve this: initialize the pointer variables before dereferencing, e.g.:\n```c\nstatic uint32_t val_holder = 0;\nuint32_t *crash_trigger = &val_holder;\n*crash_trigger = 0xDEADC0DE;\n```";
        } else if (text.toLowerCase().includes("gpio") || text.toLowerCase().includes("pin") || text.toLowerCase().includes("led")) {
          aiResponse = "To configure a GPIO pin as an output (e.g. Pin A5 for the MCU Led), enable the GPIO clock, then map the HAL parameters:\n```c\nGPIO_InitTypeDef GPIO_InitStruct = {0};\n__HAL_RCC_GPIOA_CLK_ENABLE();\n\nGPIO_InitStruct.Pin = GPIO_PIN_5;\nGPIO_InitStruct.Mode = GPIO_MODE_OUTPUT_PP;\nGPIO_InitStruct.Pull = GPIO_NOPULL;\nGPIO_InitStruct.Speed = GPIO_SPEED_FREQ_LOW;\nHAL_GPIO_Init(GPIOA, &GPIO_InitStruct);\n```";
        } else if (text.toLowerCase().includes("rag") || text.toLowerCase().includes("datasheet") || text.toLowerCase().includes("pdf")) {
          aiResponse = "I have scanned the active **RAG databases** (3 files loaded). According to the `STM32F401 Reference Manual`, the system tick timer SysTick operates off the AHB clock. Would you like me to generate a FreeRTOS delay implementation using this?";
        } else {
          aiResponse = "I have reviewed your active firmware codes. Your peripheral configurations and linker layouts (`stm32f401.ld`) are structured properly. Let me know if you would like me to compile or execute it in the emulator.";
        }
        
        workspaceStore.update(s => ({
          ...s,
          aiMessages: [
            ...s.aiMessages,
            {
              id: Math.random().toString(),
              sender: "ai",
              text: aiResponse,
              timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
            }
          ],
          aiWaiting: false
        }));
      }, 1000);

      return {
        ...state,
        aiMessages: newMsgs,
        aiWaiting: true
      };
    });
  }
};
