$ErrorActionPreference = "Stop"

$RootDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$IntegrationDir = Join-Path $RootDir "integration"

New-Item -ItemType Directory -Force -Path (Join-Path $RootDir "data") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $IntegrationDir "uploads") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $IntegrationDir "logs") | Out-Null

$RootEnvExample = Join-Path $RootDir ".env.example"
if (-not (Test-Path $RootEnvExample)) {
    Copy-Item (Join-Path $IntegrationDir "backend/.env.example") $RootEnvExample
}

Write-Host "Building rag-cli into integration/rag-cli.exe"
Push-Location $RootDir
go run build.go
Pop-Location

Write-Host ""
Write-Host "Setup complete."
Write-Host "1. Copy integration/backend/.env.example to your backend .env"
Write-Host "2. Set RAG_CLI_PATH, RAG_DB_PATH, RAG_DATA_DIR, and UPLOAD_DIR"
Write-Host "3. Put PDFs into the chosen RAG_DATA_DIR"
Write-Host "4. Run the ingest command once before querying"
