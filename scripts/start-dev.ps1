# scripts/start-dev.ps1
# Quick start script for development environment

param(
    [switch]$SkipBackend,
    [switch]$SkipFrontend,
    [switch]$Verbose
)

# Import color functions
function Write-Info { Write-Host $args[0] -ForegroundColor Cyan }
function Write-Success { Write-Host $args[0] -ForegroundColor Green }
function Write-Warning { Write-Host $args[0] -ForegroundColor Yellow }
function Write-Error { Write-Host $args[0] -ForegroundColor Red }

Write-Info "================================"
Write-Info "TaskMaster Development Start"
Write-Info "================================"

# Check if Docker is running
try {
    docker ps 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Docker is not running. Please start Docker Desktop."
        exit 1
    }
}
catch {
    Write-Error "Docker is not running. Please start Docker Desktop."
    exit 1
}

# Start infrastructure services
Write-Warning "Starting infrastructure services..."
docker-compose up -d postgres redis envoy

# Wait for services to be ready
Write-Warning "Waiting for services to be ready..."
Start-Sleep -Seconds 5

# Check service status
$services = @(
    @{Name="PostgreSQL"; Port=5432},
    @{Name="Redis"; Port=6379},
    @{Name="Envoy"; Port=8080}
)

foreach ($service in $services) {
    $tcpClient = New-Object System.Net.Sockets.TcpClient
    try {
        $tcpClient.Connect("localhost", $service.Port)
        Write-Success "$($service.Name) is ready on port $($service.Port)"
        $tcpClient.Close()
    }
    catch {
        Write-Error "$($service.Name) is not responding on port $($service.Port)"
    }
}

# Start backend if not skipped
if (-not $SkipBackend) {
    Write-Warning "Starting backend server..."
    if (Test-Path ".env") {
        Write-Info "Loading environment from .env file"
    }

    # Start backend in new PowerShell window
    $backendScript = @"
Write-Host 'TaskMaster Backend Server' -ForegroundColor Green
Write-Host '=========================' -ForegroundColor Green
Set-Location '$PWD'
go run ./cmd/server/main.go
"@

    Start-Process powershell -ArgumentList "-NoExit", "-Command", $backendScript
    Write-Success "Backend server started in new window"
}

# Start frontend if not skipped and frontend directory exists
if (-not $SkipFrontend -and (Test-Path "frontend")) {
    Write-Warning "Starting frontend development server..."

    # Start frontend in new PowerShell window
    $frontendScript = @"
Write-Host 'TaskMaster Frontend Server' -ForegroundColor Blue
Write-Host '==========================' -ForegroundColor Blue
Set-Location '$PWD\frontend'
npm run dev
"@

    Start-Process powershell -ArgumentList "-NoExit", "-Command", $frontendScript
    Write-Success "Frontend server started in new window"
}

# Display status
Write-Info "`n================================"
Write-Success "Development environment is running!"
Write-Info "================================"
Write-Host ""
Write-Host "Services:" -ForegroundColor Yellow
Write-Host "  PostgreSQL:    http://localhost:5432" -ForegroundColor Gray
Write-Host "  Redis:         http://localhost:6379" -ForegroundColor Gray
Write-Host "  Envoy Proxy:   http://localhost:8080" -ForegroundColor Gray
Write-Host "  Envoy Admin:   http://localhost:9901" -ForegroundColor Gray
Write-Host "  gRPC Backend:  http://localhost:50051" -ForegroundColor Gray
if (-not $SkipFrontend -and (Test-Path "frontend")) {
    Write-Host "  Frontend:      http://localhost:3000" -ForegroundColor Gray
}
Write-Host ""
Write-Host "Commands:" -ForegroundColor Yellow
Write-Host "  View logs:     docker-compose logs -f [service]" -ForegroundColor Gray
Write-Host "  Stop all:      .\scripts\setup-envoy.ps1 stop" -ForegroundColor Gray
Write-Host "  Clean all:     .\scripts\setup-envoy.ps1 clean" -ForegroundColor Gray
Write-Host ""

# Keep the script window open
if ($Verbose) {
    Write-Host "Press any key to exit..." -ForegroundColor Yellow
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
}