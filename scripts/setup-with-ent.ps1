# setup-with-ent.ps1
# PowerShell script to set up Ent ORM for TaskMaster
param(
    [switch]$Clean = $false,
    [switch]$SkipInstall = $false
)

$ErrorActionPreference = "Stop"

function Write-ColorOutput {
    param(
        [string]$Color,
        [string]$Message
    )
    Write-Host $Message -ForegroundColor $Color
}

Write-ColorOutput "Cyan" "Setting up Ent ORM for TaskMaster"
Write-Host ""

if ($Clean) {
    Write-ColorOutput "Yellow" "Cleaning existing Ent files..."
    if (Test-Path "ent") {
        Remove-Item -Path "ent" -Recurse -Force -Exclude "schema", "generate.go"
        Write-ColorOutput "Green" "Cleaned generated files"
    }
}

if (-not $SkipInstall) {
    Write-ColorOutput "Yellow" "Installing Ent dependencies..."

    try {
        # Install Ent CLI
        go get entgo.io/ent/cmd/ent
        go install entgo.io/ent/cmd/ent

        # Install Ent runtime and dependencies
        go get entgo.io/ent
        go get github.com/lib/pq
        go get github.com/joho/godotenv
        go get github.com/google/uuid

        Write-ColorOutput "Green" "Dependencies installed"
    }
    catch {
        Write-ColorOutput "Red" "Failed to install dependencies: $_"
        exit 1
    }
}

# Check if schema exists
if (-not (Test-Path "ent/schema/task.go")) {
    Write-ColorOutput "Yellow" "Initializing Ent schema..."

    # Create ent directory if it doesn't exist
    if (-not (Test-Path "ent")) {
        New-Item -ItemType Directory -Path "ent"
    }

    # Initialize with Task schema
    go run entgo.io/ent/cmd/ent init Task
    Write-ColorOutput "Green" "Task schema initialized"
} else {
    Write-ColorOutput "Green" "Task schema already exists"
}

# Check if generate.go exists
if (-not (Test-Path "ent/generate.go")) {
    Write-ColorOutput "Yellow" "Creating generate.go file..."

    $generateContent = @'
package ent

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./schema
'@

    Set-Content -Path "ent/generate.go" -Value $generateContent
    Write-ColorOutput "Green" "generate.go created"
} else {
    Write-ColorOutput "Green" "generate.go already exists"
}

# Generate Ent code
Write-ColorOutput "Yellow" "Generating Ent code..."
try {
    go generate ./ent
    Write-ColorOutput "Green" "Ent code generated successfully"
}
catch {
    Write-ColorOutput "Red" "Failed to generate Ent code: $_"
    Write-ColorOutput "Yellow" "Make sure your schema is valid and try again"
    exit 1
}

# Verify generated files
$expectedFiles = @(
    "ent/client.go",
    "ent/task.go",
    "ent/task_create.go",
    "ent/task_query.go",
    "ent/task_update.go",
    "ent/task_delete.go"
)

Write-ColorOutput "Yellow" "Verifying generated files..."
$allExist = $true
foreach ($file in $expectedFiles) {
    if (Test-Path $file) {
        Write-Host "   OK $file" -ForegroundColor Green
    } else {
        Write-Host "   MISSING $file" -ForegroundColor Red
        $allExist = $false
    }
}

if (-not $allExist) {
    Write-ColorOutput "Red" "Some files are missing. Please check your schema and regenerate."
    exit 1
}

# Check if .env.example exists
if (-not (Test-Path ".env.example")) {
    Write-ColorOutput "Yellow" "Creating .env.example file..."

    $envContent = @'
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=taskmaster
DB_SSLMODE=disable

# Connection Pool Settings
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_MAX_LIFETIME_MINUTES=5

# Server Configuration
GRPC_PORT=50051
AUTO_MIGRATE=true

# Logging
LOG_LEVEL=info
'@

    Set-Content -Path ".env.example" -Value $envContent
    Write-ColorOutput "Green" ".env.example created"
}

Write-Host ""
Write-ColorOutput "Green" "Ent setup completed successfully!"
Write-Host ""
Write-ColorOutput "Cyan" "Next steps:"
Write-Host "1. Copy .env.example to .env and update values:" -ForegroundColor White
Write-Host "   cp .env.example .env" -ForegroundColor Gray
Write-Host ""
Write-Host "2. Start PostgreSQL database (using Docker):" -ForegroundColor White
Write-Host "   docker-compose up -d postgres" -ForegroundColor Gray
Write-Host ""
Write-Host "3. Run database migrations:" -ForegroundColor White
Write-Host "   go run cmd/migrate/main.go" -ForegroundColor Gray
Write-Host ""
Write-Host "4. Start the server:" -ForegroundColor White
Write-Host "   go run cmd/server/main.go" -ForegroundColor Gray
Write-Host ""
Write-ColorOutput "Yellow" "Development commands:"
Write-Host "   • Regenerate Ent code: go generate ./ent"
Write-Host "   • Reset database: go run cmd/migrate/main.go -drop"
Write-Host "   • Check schema: go run cmd/migrate/main.go -dry-run"
Write-Host ""
Write-ColorOutput "Magenta" "Useful resources:"
Write-Host "   • Ent Documentation: https://entgo.io/docs/getting-started"
Write-Host "   • Schema Guide: https://entgo.io/docs/schema-def"
Write-Host "   • Migrations: https://entgo.io/docs/migrate"