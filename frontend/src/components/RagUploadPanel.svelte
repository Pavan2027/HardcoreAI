<script lang="ts">
  import { workspaceStore, actions } from "../store";
  import { 
    Upload, 
    FileText, 
    Trash2, 
    Search, 
    Sparkles, 
    BookOpen, 
    HelpCircle
  } from "lucide-svelte";

  let filesInput: HTMLInputElement;
  let isDragOver = false;
  let queryText = "";

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    isDragOver = true;
  }

  function handleDragLeave() {
    isDragOver = false;
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    isDragOver = false;
    
    if (e.dataTransfer?.files && e.dataTransfer.files.length > 0) {
      const files = Array.from(e.dataTransfer.files);
      uploadFiles(files);
    }
  }

  function handleFileSelect(e: Event) {
    const target = e.target as HTMLInputElement;
    if (target.files && target.files.length > 0) {
      const files = Array.from(target.files);
      uploadFiles(files);
    }
  }

  function uploadFiles(files: File[]) {
    files.forEach(file => {
      actions.uploadDocument(file.name, file.size);
    });
  }

  function triggerSearch() {
    actions.searchRag(queryText);
  }

  function handleKeyPress(e: KeyboardEvent) {
    if (e.key === "Enter") {
      triggerSearch();
    }
  }

  function deleteDoc(id: string, name: string) {
    workspaceStore.update(s => ({
      ...s,
      ragDocuments: s.ragDocuments.filter(d => d.id !== id)
    }));
    actions.addEmulationLog(`Purged RAG contextual document: ${name}`);
  }
</script>

<div class="rag-container">
  
  <!-- Left Side: File Drop & Vector Index Stepper -->
  <div class="rag-upload-col">
    <div class="rag-section-title">
      <Upload size={13} style="color: var(--accent-violet);" />
      <span>RAG DOCUMENT INGESTION ZONE</span>
    </div>

    <!-- Drag & Drop Zone -->
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div 
      class="drag-drop-zone {isDragOver ? 'drag-over' : ''} {$workspaceStore.ragUploadProgress ? 'disabled' : ''}"
      ondragover={handleDragOver}
      ondragleave={handleDragLeave}
      ondrop={handleDrop}
      onclick={() => !$workspaceStore.ragUploadProgress && filesInput.click()}
    >
      <input 
        type="file" 
        bind:this={filesInput} 
        onchange={handleFileSelect} 
        style="display: none;" 
        multiple
        accept=".pdf,.txt,.c,.h,.cpp,.json"
      />
      <div class="drop-icon-circle">
        <Upload size={22} style="color: var(--accent-violet);" />
      </div>
      <h3>Drag & drop hardware reference docs here</h3>
      <p>Supports STM32 Datasheets, PDF reference manuals, C registers libraries (Max 50MB)</p>
      <button class="select-files-btn" disabled={!!$workspaceStore.ragUploadProgress}>Select File</button>
    </div>

    <!-- Dynamic Ingesting Progress Tracker -->
    {#if $workspaceStore.ragUploadProgress}
      <div class="indexing-progress-card">
        <div class="spinner-group">
          <div class="pulse-ring"></div>
          <span class="status-indicator-dot"></span>
        </div>
        <div class="progress-details">
          <h4>INGESTION ENGINE ACTIVE</h4>
          <p>{$workspaceStore.ragUploadProgress}</p>
        </div>
      </div>
    {/if}

    <div class="rag-section-title" style="margin-top: 14px;">
      <BookOpen size={13} style="color: var(--accent-cyan);" />
      <span>ACTIVE CONTEXT KNOWLEDGE REGISTRY</span>
    </div>

    <!-- Active Document List -->
    <div class="rag-doc-list">
      {#each $workspaceStore.ragDocuments as doc}
        <div class="doc-card">
          <div class="doc-icon-cell">
            <FileText size={16} style="color: var(--accent-violet);" />
          </div>
          <div class="doc-info-cell">
            <span class="doc-name" title={doc.name}>{doc.name}</span>
            <div class="doc-meta">
              <span>{doc.size}</span>
              <span class="dot-separator">•</span>
              <span>{doc.chunks > 0 ? `${doc.chunks} chunks` : 'Processing...'}</span>
              {#if doc.tokens > 0}
                <span class="dot-separator">•</span>
                <span style="color: var(--accent-cyan);">{doc.tokens.toLocaleString()} tokens</span>
              {/if}
            </div>
          </div>
          <div class="doc-status-cell">
            {#if doc.status === "Ready in Database"}
              <span class="status-badge ready">Ready</span>
            {:else}
              <span class="status-badge processing">{doc.status}</span>
            {/if}
          </div>
          <button 
            class="delete-doc-btn" 
            onclick={() => deleteDoc(doc.id, doc.name)}
            title="Remove document from AI workspace context"
          >
            <Trash2 size={12} />
          </button>
        </div>
      {/each}
    </div>
  </div>

  <!-- Right Side: Semantic Snippet Search -->
  <div class="rag-search-col">
    <div class="rag-section-title">
      <Search size={13} style="color: var(--accent-success);" />
      <span>SEMANTIC RETRIEVAL TESTER</span>
    </div>

    <p class="search-desc-txt">
      Conduct dense vector matches against your uploaded hardware knowledge databases. Test if the AI extracts exact memory maps or pin functions.
    </p>

    <!-- Search Input Bar -->
    <div class="semantic-search-box">
      <Search size={13} class="search-ico" />
      <input 
        type="text" 
        placeholder="Type semantic query... (e.g. USART2 pin configurations)" 
        bind:value={queryText}
        onkeypress={handleKeyPress}
        class="semantic-input"
      />
      <button class="search-action-btn" onclick={triggerSearch}>Retrieve</button>
    </div>

    <!-- Match Results Container -->
    <div class="semantic-results-panel">
      {#if $workspaceStore.semanticResults.length === 0}
        <div class="search-placeholder-view">
          <HelpCircle size={28} style="color: var(--text-dark); stroke-width: 1.5;" />
          <p>No active semantic search results.</p>
          <span>Enter a query above to extract hardware specification chunks.</span>
        </div>
      {:else}
        <div class="results-header-info">
          <span>Found {$workspaceStore.semanticResults.length} high-probability specifications matching query</span>
        </div>
        
        <div class="results-scroller">
          {#each $workspaceStore.semanticResults as res}
            <div class="result-chunk-card">
              <div class="chunk-header">
                <span class="chunk-filename">{res.file}</span>
                <span class="chunk-score">Relevance: {(res.score * 100).toFixed(0)}%</span>
              </div>
              <div class="chunk-body">
                "{res.match}"
              </div>
              <div class="chunk-footer">
                <Sparkles size={10} style="color: var(--accent-cyan);" />
                <span>Indexed inside vector store memory</span>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .rag-container {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
    width: 100%;
    background: #09090D;
    color: var(--text-light);
    font-family: var(--font-sans);
    box-sizing: border-box;
    overflow-y: auto;
    padding: 8px;
  }

  .rag-upload-col {
    flex: 1.1;
    display: flex;
    flex-direction: column;
    gap: 10px;
    background: #0E0E14;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 12px;
    box-sizing: border-box;
  }

  .rag-search-col {
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

  .rag-section-title {
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

  .drag-drop-zone {
    border: 2px dashed var(--border-color);
    border-radius: var(--radius-md);
    background: #0B0B0F;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 24px 16px;
    text-align: center;
    cursor: pointer;
    transition: all 0.22s ease;
  }

  .drag-drop-zone:hover:not(.disabled) {
    border-color: var(--accent-violet);
    background: rgba(139, 92, 246, 0.02);
  }

  .drag-drop-zone.drag-over {
    border-color: var(--accent-cyan);
    background: rgba(6, 182, 212, 0.05);
  }

  .drag-drop-zone.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .drop-icon-circle {
    width: 44px;
    height: 44px;
    border-radius: 50%;
    background: #151522;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 10px;
  }

  .drag-drop-zone h3 {
    margin: 0 0 6px 0;
    font-size: 0.82rem;
    color: var(--text-bright);
    font-weight: 600;
  }

  .drag-drop-zone p {
    margin: 0 0 12px 0;
    font-size: 0.65rem;
    color: var(--text-dark);
    max-width: 280px;
    line-height: 1.3;
  }

  .select-files-btn {
    background: #14141E;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    color: var(--text-light);
    font-size: 0.72rem;
    font-weight: 500;
    padding: 4px 14px;
    cursor: pointer;
  }

  .select-files-btn:hover:not(:disabled) {
    background: #1D1D2C;
    border-color: var(--accent-violet);
  }

  .indexing-progress-card {
    display: flex;
    align-items: center;
    gap: 12px;
    background: rgba(139, 92, 246, 0.05);
    border: 1px dashed var(--accent-violet);
    border-radius: var(--radius-sm);
    padding: 8px 12px;
  }

  .spinner-group {
    position: relative;
    width: 14px;
    height: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .status-indicator-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent-violet);
    z-index: 2;
  }

  .pulse-ring {
    position: absolute;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    border: 1.5px solid var(--accent-violet);
    animation: ringPulse 1.5s infinite linear;
    z-index: 1;
  }

  @keyframes ringPulse {
    from {
      transform: scale(0.6);
      opacity: 1;
    }
    to {
      transform: scale(1.6);
      opacity: 0;
    }
  }

  .progress-details h4 {
    margin: 0 0 2px 0;
    font-size: 0.68rem;
    font-weight: 700;
    color: var(--accent-violet-hover);
    letter-spacing: 0.4px;
  }

  .progress-details p {
    margin: 0;
    font-size: 0.65rem;
    color: var(--text-light);
  }

  .rag-doc-list {
    flex-grow: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .doc-card {
    display: flex;
    align-items: center;
    background: #111116;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 6px 10px;
    gap: 10px;
  }

  .doc-icon-cell {
    display: flex;
    align-items: center;
  }

  .doc-info-cell {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .doc-name {
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--text-bright);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .doc-meta {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 0.6rem;
    color: var(--text-dark);
  }

  .dot-separator {
    color: var(--border-color);
  }

  .doc-status-cell {
    display: flex;
    align-items: center;
  }

  .status-badge {
    font-size: 0.58rem;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 10px;
  }

  .status-badge.ready {
    background: rgba(16, 185, 129, 0.08);
    color: var(--accent-success);
    border: 1px solid rgba(16, 185, 129, 0.2);
  }

  .status-badge.processing {
    background: rgba(245, 158, 11, 0.08);
    color: #F59E0B;
    border: 1px solid rgba(245, 158, 11, 0.2);
  }

  .delete-doc-btn {
    background: none;
    border: none;
    color: var(--text-dark);
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .delete-doc-btn:hover {
    color: var(--accent-error);
  }

  .search-desc-txt {
    font-size: 0.7rem;
    color: var(--text-muted);
    line-height: 1.35;
    margin: 0 0 4px 0;
  }

  .semantic-search-box {
    display: flex;
    align-items: center;
    background: #08080C;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 4px 8px;
    gap: 8px;
  }

  .semantic-input {
    background: none;
    border: none;
    color: var(--text-bright);
    font-family: var(--font-sans);
    font-size: 0.72rem;
    flex-grow: 1;
    outline: none;
  }

  .search-action-btn {
    background: var(--accent-violet);
    border: none;
    border-radius: var(--radius-sm);
    color: white;
    font-size: 0.68rem;
    font-weight: 500;
    padding: 3px 10px;
    cursor: pointer;
  }

  .search-action-btn:hover {
    background: var(--accent-violet-hover);
  }

  .semantic-results-panel {
    flex-grow: 1;
    background: #08080C;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 10px;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .search-placeholder-view {
    margin: auto;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 8px;
    max-width: 220px;
  }

  .search-placeholder-view p {
    margin: 0;
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--text-muted);
  }

  .search-placeholder-view span {
    font-size: 0.65rem;
    color: var(--text-dark);
    line-height: 1.3;
  }

  .results-header-info {
    font-size: 0.65rem;
    font-weight: 600;
    color: var(--text-muted);
    border-bottom: 1px solid #1A1A26;
    padding-bottom: 6px;
    margin-bottom: 8px;
  }

  .results-scroller {
    flex-grow: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .result-chunk-card {
    background: #111116;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .chunk-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.62rem;
    font-weight: 600;
  }

  .chunk-filename {
    color: var(--accent-violet-hover);
  }

  .chunk-score {
    color: var(--accent-success);
    background: rgba(16, 185, 129, 0.05);
    padding: 1px 4px;
    border-radius: 3px;
  }

  .chunk-body {
    font-size: 0.68rem;
    color: var(--text-light);
    line-height: 1.35;
    font-family: var(--font-mono);
    background: #08080C;
    border-radius: var(--radius-sm);
    padding: 6px;
  }

  .chunk-footer {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 0.58rem;
    color: var(--text-dark);
  }
</style>
