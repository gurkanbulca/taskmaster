# scripts/generate-from-proto.ps1
# Generate Go code from proto submodule - REFACTORED VERSION

$ErrorActionPreference = "Stop"

Write-Host "🔄 Generating Go code from proto submodule..." -ForegroundColor Yellow

# --- CONFIGURATION ---
# Define the services to be processed. Add new service names here.
$services = "auth", "task", "common"

# Define base paths to keep the script clean.
$baseApiProtoPath = Join-Path $PSScriptRoot "..\api\proto"
$baseProtoGenPath = Join-Path $PSScriptRoot "..\proto\gen\go"

# --- EXECUTION ---

# 1. Update submodule to the latest version.
Write-Host "Updating proto submodule..." -ForegroundColor Cyan
git submodule update --remote proto

# 2. Clean old generated code and create fresh directories.
Write-Host "Cleaning old generated code..." -ForegroundColor Cyan
foreach ($service in $services) {
    $generatedPath = Join-Path $baseApiProtoPath "$service\v1\generated"
    if (Test-Path $generatedPath) {
        Remove-Item -Path $generatedPath -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path $generatedPath | Out-Null
}

# 3. Generate Go code from the submodule.
Write-Host "Generating code in proto submodule..." -ForegroundColor Cyan
Push-Location proto

# Check which script exists and run it.
if (Test-Path "build.ps1") {
    Write-Verbose "Running proto build.ps1..."
    & .\build.ps1 -Task "generate-go"
} elseif (Test-Path "scripts\generate-go.ps1") {
    Write-Verbose "Running proto generate-go.ps1..."
    & .\scripts\generate-go.ps1
} else {
    Write-Host "❌ No proto generation script found!" -ForegroundColor Red
    Pop-Location
    exit 1
}

Pop-Location

# 4. Verify that the generation was successful.
if (!(Test-Path $baseProtoGenPath)) {
    Write-Host "❌ Proto generation failed - '$($baseProtoGenPath)' directory not found!" -ForegroundColor Red
    exit 1
}

# 5. Copy the newly generated code to the backend's API structure.
Write-Host "Copying generated code..." -ForegroundColor Cyan
foreach ($service in $services) {
    $sourcePath = Join-Path $baseProtoGenPath "$service\v1"
    $destinationPath = Join-Path $baseApiProtoPath "$service\v1\generated"

    if (Test-Path $sourcePath) {
        Copy-Item -Path "$sourcePath\*" -Destination $destinationPath -Recurse -Force
        Write-Host "  ✓ Copied $service/v1 generated files" -ForegroundColor Green
    }
}

Write-Host "✅ Go code generated from proto submodule!" -ForegroundColor Green