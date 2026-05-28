<script lang="ts">
  let problem = "";
  let chipFamily = "STM32F4";
  let llmProvider = "openrouter";
  let files: File[] = [];
  let loading = false;
  let responseText = "";
  let error = "";

  function handleFileChange(event: Event) {
    const target = event.currentTarget as HTMLInputElement;
    files = Array.from(target.files ?? []);
  }

  async function solve() {
    loading = true;
    error = "";
    responseText = "";

    const formData = new FormData();
    formData.append("problem", problem);
    formData.append("llm_provider", llmProvider);
    formData.append("chip_family", chipFamily);
    files.forEach((file) => formData.append("documents", file));

    try {
      const response = await fetch("/api/projects/demo/agent/solve", {
        method: "POST",
        body: formData
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.detail ?? "Request failed");
      }
      responseText = payload.rag_context || payload.raw_stdout || "No context returned.";
    } catch (err) {
      error = err instanceof Error ? err.message : "Unknown error";
    } finally {
      loading = false;
    }
  }
</script>

<section class="panel">
  <div class="heading">
    <h2>RAG Solve</h2>
    <p>Attach PDFs, describe the hardware task, and fetch retrieval context for your coding agent.</p>
  </div>

  <label>
    Problem
    <textarea bind:value={problem} rows="6" placeholder="Example: Configure PA5 as a push-pull GPIO output for LED blink." />
  </label>

  <div class="grid">
    <label>
      LLM Provider
      <input bind:value={llmProvider} />
    </label>
    <label>
      Chip Family
      <input bind:value={chipFamily} />
    </label>
  </div>

  <label class="upload">
    Documents
    <input type="file" accept=".pdf" multiple on:change={handleFileChange} />
  </label>

  {#if files.length}
    <ul>
      {#each files as file}
        <li>{file.name}</li>
      {/each}
    </ul>
  {/if}

  <button on:click={solve} disabled={loading || !problem.trim()}>
    {#if loading}Running retrieval...{:else}Solve{/if}
  </button>

  {#if error}
    <pre class="error">{error}</pre>
  {/if}

  {#if responseText}
    <pre>{responseText}</pre>
  {/if}
</section>

<style>
  :global(body) {
    font-family: "IBM Plex Sans", system-ui, sans-serif;
    background:
      radial-gradient(circle at top left, rgba(255, 180, 70, 0.18), transparent 30%),
      linear-gradient(180deg, #f5efe3 0%, #f3f6fb 100%);
    color: #142033;
  }

  .panel {
    max-width: 900px;
    margin: 2rem auto;
    padding: 1.5rem;
    border: 1px solid rgba(20, 32, 51, 0.1);
    border-radius: 20px;
    background: rgba(255, 255, 255, 0.86);
    box-shadow: 0 24px 60px rgba(20, 32, 51, 0.1);
  }

  .heading h2 {
    margin: 0 0 0.25rem;
    font-size: 2rem;
  }

  .heading p {
    margin: 0 0 1.25rem;
    color: #4e5b72;
  }

  label {
    display: block;
    margin-bottom: 1rem;
    font-weight: 600;
  }

  textarea,
  input {
    width: 100%;
    margin-top: 0.4rem;
    padding: 0.8rem 0.9rem;
    border: 1px solid #cfd8e3;
    border-radius: 12px;
    font: inherit;
    box-sizing: border-box;
    background: #fff;
  }

  .grid {
    display: grid;
    gap: 1rem;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  }

  button {
    margin-top: 0.5rem;
    border: 0;
    border-radius: 999px;
    padding: 0.9rem 1.3rem;
    font: inherit;
    font-weight: 700;
    color: white;
    background: linear-gradient(135deg, #0f766e, #2563eb);
    cursor: pointer;
  }

  pre {
    margin-top: 1rem;
    padding: 1rem;
    border-radius: 14px;
    overflow: auto;
    background: #132033;
    color: #edf3ff;
  }

  .error {
    background: #4a0f16;
  }
</style>
