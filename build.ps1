# build.ps1
# Main build script for backend (Windows) - FIXED VERSION

param(
    [Parameter(Mandatory=$false)]
    [ValidateSet("proto", "update-proto", "ent", "generate", "run", "test", "build", "docker", "clean", "help")]
    [string]$Task = "help"
)

$ErrorActionPreference = "Stop"

# Set script root
$ScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Definition

function Show-Help {
    Write-Host ""
    Write-Host "TaskMaster Backend Build Script" -ForegroundColor Cyan
    Write-Host "================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage: .\build.ps1 -Task <task>" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Available tasks:" -ForegroundColor Green
    Write-Host "  proto        - Generate Go code from proto submodule"
    Write-Host "  update-proto - Update proto submodule to latest"
    Write-Host "  ent          - Generate Ent ORM code"
    Write-Host "  generate     - Generate all code (Proto + Ent)"
    Write-Host "  run          - Run the server"
    Write-Host "  test         - Run tests"
    Write-Host "  build        - Build the server binary"
    Write-Host "  docker       - Build Docker image"
    Write-Host "  clean        - Clean generated files"
    Write-Host "  help         - Show this help message"
    Write-Host ""
    Write-Host "Examples:" -ForegroundColor Yellow
    Write-Host "  .\build.ps1 -Task generate"
    Write-Host "  .\build.ps1 -Task run"
    Write-Host ""
}

function Invoke-Proto {
    Write-Host "📦 Generating Proto code..." -ForegroundColor Yellow
    $scriptPath = Join-Path $ScriptRoot "scripts\generate-from-proto.ps1"
    if (Test-Path $scriptPath) {
        & $scriptPath
    } else {
        Write-Host "❌ Script not found: $scriptPath" -ForegroundColor Red
        exit 1
    }
}

function Update-Proto {
    Write-Host "🔄 Updating proto submodule..." -ForegroundColor Yellow
    git submodule update --remote proto
    Push-Location proto
    git pull origin main
    Pop-Location
    Write-Host "✅ Proto submodule updated!" -ForegroundColor Green
}

function Invoke-Ent {
    Write-Host "📦 Generating Ent code..." -ForegroundColor Yellow
    go generate ./ent
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Ent generation failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "✅ Ent code generated!" -ForegroundColor Green
}

function Invoke-Generate {
    Write-Host "🚀 Generating all code..." -ForegroundColor Magenta
    Invoke-Proto
    Write-Host ""
    Invoke-Ent
    Write-Host "✅ All code generated!" -ForegroundColor Green
}

function Invoke-Run {
    Write-Host "🚀 Starting server..." -ForegroundColor Green
    go run cmd/server/main.go
}

function Invoke-Test {
    Write-Host "🧪 Running tests..." -ForegroundColor Yellow
    go test -v -race ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Tests failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "✅ All tests passed!" -ForegroundColor Green
}

function Invoke-Build {
    Write-Host "🔨 Building server..." -ForegroundColor Yellow
    $outputPath = Join-Path $ScriptRoot "bin\server.exe"
    go build -o $outputPath cmd/server/main.go
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Build failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "✅ Server built: $outputPath" -ForegroundColor Green
}

function Invoke-Docker {
    Write-Host "🐳 Building Docker image..." -ForegroundColor Yellow
    docker build -t taskmaster-backend .
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Docker build failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "✅ Docker image built!" -ForegroundColor Green
}

function Invoke-Clean {
    Write-Host "🧹 Cleaning generated files..." -ForegroundColor Yellow

    # Clean proto generated files
    $authGenPath = Join-Path $ScriptRoot "api\proto\auth\v1\generated"
    $taskGenPath = Join-Path $ScriptRoot "api\proto\task\v1\generated"
    $commonGenPath = Join-Path $ScriptRoot "api\proto\common\v1\generated"
    $entGenPath = Join-Path $ScriptRoot "ent\generated"
    $protoGenPath = Join-Path $ScriptRoot "proto\gen"

    @($authGenPath, $taskGenPath, $commonGenPath, $entGenPath, $protoGenPath) | ForEach-Object {
        if (Test-Path $_) {
            Remove-Item -Path $_ -Recurse -Force
            Write-Host "  ✓ Removed $_" -ForegroundColor Green
        }
    }

    Write-Host "✅ Clean complete!" -ForegroundColor Green
}

# Main switch
switch ($Task) {
    "proto" { Invoke-Proto }
    "update-proto" { Update-Proto }
    "ent" { Invoke-Ent }
    "generate" { Invoke-Generate }
    "run" { Invoke-Run }
    "test" { Invoke-Test }
    "build" { Invoke-Build }
    "docker" { Invoke-Docker }
    "clean" { Invoke-Clean }
    "help" { Show-Help }
    default { Show-Help }
}