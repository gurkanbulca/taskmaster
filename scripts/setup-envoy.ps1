# scripts/setup-envoy.ps1
# Setup and manage Envoy proxy for gRPC-Web on Windows

param(
    [Parameter(Position = 0)]
    [ValidateSet("dev", "prod", "test", "logs", "stop", "clean", "status")]
    [string]$Mode = "dev",

    [Parameter()]
    [switch]$UseDockerBackend
)

# Color functions for output
function Write-ColorOutput {
    param([string]$Message, [string]$Color = "White")
    Write-Host $Message -ForegroundColor $Color
}

function Write-Info { Write-ColorOutput $args[0] "Cyan" }
function Write-Success { Write-ColorOutput $args[0] "Green" }
function Write-Warning { Write-ColorOutput $args[0] "Yellow" }
function Write-Error { Write-ColorOutput $args[0] "Red" }

# Banner
Write-Info "================================"
Write-Info "TaskMaster Envoy Setup"
Write-Info "================================"

# Function to check if a command exists
function Test-CommandExists {
    param([string]$Command)
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

# Function to test if a port is open
function Test-Port {
    param(
        [string]$HostName = "localhost",
        [int]$Port,
        [int]$Timeout = 1
    )
    try {
        $tcpClient = New-Object System.Net.Sockets.TcpClient
        $connect = $tcpClient.BeginConnect($HostName, $Port, $null, $null)
        $wait = $connect.AsyncWaitHandle.WaitOne($Timeout * 1000, $false)

        if ($wait) {
            $tcpClient.EndConnect($connect)
            $tcpClient.Close()
            return $true
        }
        else {
            $tcpClient.Close()
            return $false
        }
    }
    catch {
        return $false
    }
}

# Function to wait for a service to be ready
function Wait-ForService {
    param(
        [string]$ServiceName,
        [string]$HostName = "localhost",
        [int]$Port,
        [int]$MaxAttempts = 30
    )

    Write-Warning "Waiting for $ServiceName to be ready on ${HostName}:${Port}..."

    for ($i = 0; $i -lt $MaxAttempts; $i++) {
        if (Test-Port -Host $HostName -Port $Port) {
            Write-Success "$ServiceName is ready!"
            return $true
        }
        Write-Host "." -NoNewline
        Start-Sleep -Seconds 2
    }

    Write-Error "$ServiceName failed to start on ${HostName}:${Port}"
    return $false
}

# Check prerequisites
function Test-Prerequisites {
    Write-Warning "Checking prerequisites..."

    $hasDocker = Test-CommandExists "docker"
    $hasDockerCompose = Test-CommandExists "docker-compose"

    if (-not $hasDocker) {
        Write-Error "Docker is not installed or not in PATH. Please install Docker Desktop for Windows."
        Write-Info "Download from: https://www.docker.com/products/docker-desktop"
        return $false
    }

    if (-not $hasDockerCompose) {
        Write-Error "Docker Compose is not installed or not in PATH."
        Write-Info "Docker Desktop for Windows includes Docker Compose. Make sure it's enabled."
        return $false
    }

    # Check if Docker is running
    try {
        docker ps 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Docker is installed but not running. Please start Docker Desktop."
            return $false
        }
    }
    catch {
        Write-Error "Docker is not running. Please start Docker Desktop."
        return $false
    }

    Write-Success "Prerequisites satisfied!"
    return $true
}

# Create necessary directories
function Initialize-Directories {
    Write-Warning "Creating directory structure..."

    $directories = @(
        "logs",
        "deployments/envoy",
        "deployments/docker"
    )

    foreach ($dir in $directories) {
        if (-not (Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
            Write-Info "Created directory: $dir"
        }
    }
}

# Main execution
switch ($Mode) {
    "dev" {
        if (-not (Test-Prerequisites)) { exit 1 }
        Initialize-Directories

        Write-Info "Starting development environment..."

        # Start PostgreSQL and Redis
        Write-Warning "Starting PostgreSQL and Redis..."
        docker-compose up -d postgres redis

        # Wait for services
        Wait-ForService -ServiceName "PostgreSQL" -Port 5432
        Wait-ForService -ServiceName "Redis" -Port 6379

        # Handle backend
        if ($UseDockerBackend) {
            Write-Warning "Starting gRPC backend in Docker..."
            docker-compose up -d backend
            Wait-ForService -ServiceName "gRPC Backend" -Port 50051
        }
        else {
            Write-Warning "Please start the gRPC backend locally in another terminal:"
            Write-Success "go run ./cmd/server/main.go"
            Write-Warning "Press Enter when the backend is running..."
            Read-Host

            if (-not (Test-Port -Port 50051)) {
                Write-Error "Backend doesn't seem to be running on port 50051"
                Write-Warning "Continue anyway? (y/n)"
                $continue = Read-Host
                if ($continue -ne "y") { exit 1 }
            }
        }

        # Start Envoy
        Write-Warning "Starting Envoy proxy..."
        docker-compose up -d envoy
        Wait-ForService -ServiceName "Envoy Proxy" -Port 8080
        Wait-ForService -ServiceName "Envoy Admin" -Port 9901

        Write-Success "Development environment is ready!"
        Write-Info "Services:"
        Write-Host "  - PostgreSQL: localhost:5432" -ForegroundColor Gray
        Write-Host "  - Redis: localhost:6379" -ForegroundColor Gray
        Write-Host "  - gRPC Backend: localhost:50051" -ForegroundColor Gray
        Write-Host "  - Envoy Proxy: localhost:8080" -ForegroundColor Gray
        Write-Host "  - Envoy Admin: localhost:9901" -ForegroundColor Gray
    }

    "prod" {
        if (-not (Test-Prerequisites)) { exit 1 }
        Initialize-Directories

        Write-Info "Starting production environment..."

        # Use production Envoy config if it exists
        if (Test-Path "deployments/envoy/envoy.prod.yaml") {
            Copy-Item "deployments/envoy/envoy.prod.yaml" "deployments/envoy/envoy.yaml" -Force
            Write-Info "Using production Envoy configuration"
        }

        # Start all services
        docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

        # Wait for services
        Wait-ForService -ServiceName "PostgreSQL" -Port 5432
        Wait-ForService -ServiceName "Redis" -Port 6379
        Wait-ForService -ServiceName "gRPC Backend" -Port 50051
        Wait-ForService -ServiceName "Envoy Proxy" -Port 8080

        Write-Success "Production environment is ready!"
    }

    "test" {
        Write-Info "Testing Envoy connection..."

        # Test if Envoy is running
        if (-not (Test-Port -Port 8080)) {
            Write-Error "Envoy is not running on port 8080"
            Write-Warning "Run './scripts/setup-envoy.ps1 dev' first"
            exit 1
        }

        # Test health endpoint (if configured)
        Write-Warning "Testing health endpoint..."
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:8080/health" -Method GET -ErrorAction SilentlyContinue
            if ($response.StatusCode -eq 200) {
                Write-Success "Health check passed"
            }
        }
        catch {
            Write-Warning "Health endpoint not configured or not responding"
        }

        # Test gRPC-Web endpoint
        Write-Warning "Testing gRPC-Web endpoint..."
        try {
            $headers = @{
                "Content-Type" = "application/grpc-web-text"
                "X-Grpc-Web" = "1"
            }
            $response = Invoke-WebRequest -Uri "http://localhost:8080/auth.v1.AuthService/Login" -Method POST -Headers $headers -ErrorAction SilentlyContinue
        }
        catch {
            # Expected to fail without proper auth, but shows connectivity
            if ($_.Exception.Response.StatusCode -eq 400 -or $_.Exception.Response.StatusCode -eq 415) {
                Write-Success "gRPC-Web endpoint is responding (auth required)"
            }
            else {
                Write-Warning "gRPC-Web endpoint returned: $($_.Exception.Message)"
            }
        }

        # Check Envoy admin stats
        Write-Warning "Checking Envoy admin interface..."
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:9901/stats" -Method GET
            Write-Success "Envoy admin interface is accessible"
            Write-Info "First 10 stats:"
            ($response.Content -split "`n")[0..9] | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
        }
        catch {
            Write-Error "Could not access Envoy admin interface"
        }

        Write-Success "Test complete!"
    }

    "logs" {
        Write-Info "Showing Envoy logs..."
        docker-compose logs -f envoy
    }

    "status" {
        Write-Info "Checking service status..."

        $services = @(
            @{Name="PostgreSQL"; Port=5432},
            @{Name="Redis"; Port=6379},
            @{Name="gRPC Backend"; Port=50051},
            @{Name="Envoy Proxy"; Port=8080},
            @{Name="Envoy Admin"; Port=9901}
        )

        foreach ($service in $services) {
            if (Test-Port -Port $service.Port) {
                Write-Success "$($service.Name) is running on port $($service.Port)"
            }
            else {
                Write-Warning "$($service.Name) is not running on port $($service.Port)"
            }
        }

        Write-Info "`nDocker containers:"
        docker-compose ps
    }

    "stop" {
        Write-Info "Stopping all services..."
        docker-compose down
        Write-Success "All services stopped!"
    }

    "clean" {
        Write-Warning "This will remove all containers and volumes. Continue? (y/n)"
        $confirm = Read-Host
        if ($confirm -eq "y") {
            Write-Info "Cleaning up everything..."
            docker-compose down -v
            Write-Success "Cleanup complete!"
        }
        else {
            Write-Info "Cleanup cancelled"
        }
    }

    default {
        Write-Error "Invalid mode: $Mode"
        Write-Info "Usage: .\scripts\setup-envoy.ps1 [dev|prod|test|logs|status|stop|clean] [-UseDockerBackend]"
        Write-Host ""
        Write-Host "Modes:" -ForegroundColor Yellow
        Write-Host "  dev    - Start development environment" -ForegroundColor Gray
        Write-Host "  prod   - Start production environment" -ForegroundColor Gray
        Write-Host "  test   - Test Envoy connectivity" -ForegroundColor Gray
        Write-Host "  logs   - Show Envoy logs" -ForegroundColor Gray
        Write-Host "  status - Check service status" -ForegroundColor Gray
        Write-Host "  stop   - Stop all services" -ForegroundColor Gray
        Write-Host "  clean  - Stop and remove all containers and volumes" -ForegroundColor Gray
        Write-Host ""
        Write-Host "Options:" -ForegroundColor Yellow
        Write-Host "  -UseDockerBackend - Run backend in Docker (for dev mode)" -ForegroundColor Gray
        exit 1
    }
}

# Show Envoy admin interface URL
if ($Mode -notin @("stop", "clean")) {
    Write-Info "================================"
    Write-Success "Envoy Admin Interface: http://localhost:9901"
    Write-Host "  View clusters: http://localhost:9901/clusters" -ForegroundColor Gray
    Write-Host "  View stats: http://localhost:9901/stats" -ForegroundColor Gray
    Write-Host "  View config: http://localhost:9901/config_dump" -ForegroundColor Gray
    Write-Info "================================"
}