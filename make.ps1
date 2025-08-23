# make.ps1 - Windows PowerShell build script
param(
    [Parameter(Position=0)]
    [string]$Target = "help"
)

switch ($Target) {
    "help" {
        Write-Host "Available commands:"
        Write-Host "  .\make.ps1 proto        - Generate Go code from proto submodule"
        Write-Host "  .\make.ps1 update-proto - Update proto submodule to latest"
        Write-Host "  .\make.ps1 generate     - Generate all code (Ent + Proto)"
        Write-Host "  .\make.ps1 run          - Run the server"
        Write-Host "  .\make.ps1 test         - Run tests"
        Write-Host "  .\make.ps1 build        - Build the server"
    }
    "proto" {
        & .\scripts\generate-from-proto.ps1
    }
    "update-proto" {
        Write-Host "Updating proto submodule..."
        git submodule update --remote proto
        Push-Location proto
        git pull origin main
        Pop-Location
        Write-Host "Proto submodule updated!"
    }
    "generate" {
        & .\scripts\generate-from-proto.ps1
        Write-Host "Generating Ent code..."
        go generate ./ent
        Write-Host "All code generated!"
    }
    "run" {
        go run cmd/server/main.go
    }
    "test" {
        go test -v -race ./...
    }
    "build" {
        go build -o bin/server.exe cmd/server/main.go
    }
    "docker-build" {
        docker build -t taskmaster-backend .
    }
    "docker-run" {
        docker-compose up -d
    }
    default {
        Write-Host "Unknown target: $Target" -ForegroundColor Red
        & $PSCommandPath help
    }
}
