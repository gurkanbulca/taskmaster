# PowerShell build script for Windows
param(
    [string]$Target = "help"
)

$ErrorActionPreference = "Stop"

function Write-ColorOutput {
    param(
        [string]$Color,
        [string]$Message
    )
    Write-Host $Message -ForegroundColor $Color
}

switch ($Target) {
    "help" {
        Write-ColorOutput "Cyan" "Available commands:"
        Write-Host "  .\build.ps1 setup      - Install dependencies"
        Write-Host "  .\build.ps1 proto      - Generate protobuf code"
        Write-Host "  .\build.ps1 build      - Build the binary"
        Write-Host "  .\build.ps1 run        - Run the server"
        Write-Host "  .\build.ps1 test       - Run tests"
        Write-Host "  .\build.ps1 clean      - Clean build artifacts"
        Write-Host "  .\build.ps1 docker-up  - Start Docker containers"
        Write-Host "  .\build.ps1 docker-down - Stop Docker containers"
    }

    "setup" {
        Write-ColorOutput "Yellow" "Installing dependencies..."
        go mod download
        go mod tidy
        Write-ColorOutput "Green" "Setup complete"
    }

    "proto" {
        Write-ColorOutput "Yellow" "Generating protobuf code..."
        $protoPath = "api/proto/task/v1/task.proto"
        if (Test-Path $protoPath) {
            $protoFile = $protoPath.Replace("\", "/")
            $cmd = "protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative $protoFile"
            Invoke-Expression $cmd
            Write-ColorOutput "Green" "Protobuf generation complete"
        } else {
            Write-ColorOutput "Red" "Proto file not found: $protoPath"
        }
    }

    "build" {
        Write-ColorOutput "Yellow" "Building binary..."
        $outputPath = "bin\server.exe"
        $sourcePath = "cmd\server\main.go"

        if (Test-Path $sourcePath) {
            go build -o $outputPath $sourcePath
            Write-ColorOutput "Green" "Build complete: $outputPath"
        } else {
            Write-ColorOutput "Red" "Source file not found: $sourcePath"
        }
    }

    "run" {
        Write-ColorOutput "Yellow" "Starting server..."
        $mainPath = "cmd\server\main.go"
        if (Test-Path $mainPath) {
            go run $mainPath
        } else {
            Write-ColorOutput "Red" "Main file not found: $mainPath"
        }
    }

    "test" {
        Write-ColorOutput "Yellow" "Running tests..."
        go test -v -race ./...
        Write-ColorOutput "Green" "Tests complete"
    }

    "clean" {
        Write-ColorOutput "Yellow" "Cleaning..."
        if (Test-Path "bin") {
            Remove-Item -Path "bin" -Recurse -Force
        }
        if (Test-Path "tmp") {
            Remove-Item -Path "tmp" -Recurse -Force
        }
        Remove-Item -Path "*.out" -Force -ErrorAction SilentlyContinue
        Remove-Item -Path "*.exe" -Force -ErrorAction SilentlyContinue
        Write-ColorOutput "Green" "Clean complete"
    }

    "docker-up" {
        Write-ColorOutput "Yellow" "Starting Docker containers..."
        docker-compose up -d
        Write-ColorOutput "Green" "Containers started"
    }

    "docker-down" {
        Write-ColorOutput "Yellow" "Stopping Docker containers..."
        docker-compose down
        Write-ColorOutput "Green" "Containers stopped"
    }

    default {
        Write-ColorOutput "Red" "Unknown target: $Target"
        Write-Host "Use '.\build.ps1 help' to see available commands"
    }
}