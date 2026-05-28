# HARDCOREAI - Embedded Developer Workspace

A premium, modern embedded developer workspace built with Svelte 5, TypeScript, and Vite. Optimize compilation, flashing, and debug loops directly on target microcontrollers with zero unnecessary visual noise.

## Tech Stack
- **Framework**: [Svelte 5](https://svelte.dev/)
- **Language**: [TypeScript](https://www.typescriptlang.org/)
- **Build Tool**: [Vite](https://vite.dev/)
- **Icons**: [Lucide Svelte](https://lucide.dev/guide/svelte)

---

## Getting Started

### 1. Prerequisites
Ensure you have [Node.js](https://nodejs.org/) installed.

### 2. Installation
Install project dependencies:
```bash
npm install
```

### 3. Development Server
Start the local development server:
```bash
npm run dev
```
Open **[http://localhost:62016/](http://localhost:62016/)** in your browser to run and interact with the application.

---

## Commands & Testing

### Type & Svelte Diagnostic Checks
Verify that the code has zero syntax or TypeScript errors:
```bash
npx svelte-check
```

### Production Build
Compile and bundle the application:
```bash
npm run build
```
The optimized bundle will be created inside the `dist/` directory.
