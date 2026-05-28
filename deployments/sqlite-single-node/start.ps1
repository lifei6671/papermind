param(
  [int]$Port = 9080,
  [string]$DatabaseFile = "server/data/sqlite/papermind.db"
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..\..")
$serverDir = Join-Path $repoRoot "server"
$binDir = Join-Path $scriptDir "bin"
$binaryPath = Join-Path $binDir "papermind-sqlite.exe"

Set-Location $repoRoot

@(
  "server/data/sqlite",
  "server/data/tmp",
  "server/data/imports",
  "server/data/exports",
  "deployments/sqlite-single-node/bin"
) | ForEach-Object {
  New-Item -ItemType Directory -Force -Path (Join-Path $repoRoot $_) | Out-Null
}

if ([System.IO.Path]::IsPathRooted($DatabaseFile)) {
  $resolvedDatabaseFile = $DatabaseFile
} else {
  $resolvedDatabaseFile = Join-Path $repoRoot $DatabaseFile
}
$storageTempDir = Join-Path $repoRoot "server/data/tmp"
$storageImportDir = Join-Path $repoRoot "server/data/imports"
$storageExportDir = Join-Path $repoRoot "server/data/exports"

$sqliteDsn = "file:$resolvedDatabaseFile`?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000"

$env:PAPERMIND_APP_ENV = "dev"
$env:PAPERMIND_APP_HTTP_PORT = [string]$Port
$env:PAPERMIND_DATABASE_DRIVER = "sqlite"
$env:PAPERMIND_DATABASE_DSN = $sqliteDsn
$env:PAPERMIND_DATABASE_MAX_OPEN_CONNS = "1"
$env:PAPERMIND_DATABASE_MAX_IDLE_CONNS = "1"
$env:PAPERMIND_STORAGE_TEMP_DIR = $storageTempDir
$env:PAPERMIND_STORAGE_IMPORT_DIR = $storageImportDir
$env:PAPERMIND_STORAGE_EXPORT_DIR = $storageExportDir

Write-Host "PaperMind SQLite 单机模式"
Write-Host "DSN: $sqliteDsn"
Write-Host "HTTP Port: $Port"
Write-Host "说明: SQLite 仅建议演示、本地开发和低并发单机使用，不推荐 100 人正式考试。"

Push-Location $serverDir
try {
  go build -tags json1 -o $binaryPath ./cmd/papermind
  & $binaryPath
} finally {
  Pop-Location
}
