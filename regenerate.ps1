# regenerate.ps1 - Complete regeneration script for Phase 2

Write-Host "Regenerating TaskMaster with Phase 2 features..." -ForegroundColor Yellow
Write-Host ""

# Step 1: Clean generated directories
Write-Host "Cleaning generated directories..." -ForegroundColor Cyan
if (Test-Path "ent/generated") {
    Remove-Item -Path "ent/generated" -Recurse -Force
    Write-Host "  Cleaned ent/generated" -ForegroundColor Green
}

if (Test-Path "api/proto/auth/v1/generated") {
    Remove-Item -Path "api/proto/auth/v1/generated" -Recurse -Force
    Write-Host "  Cleaned api/proto/auth/v1/generated" -ForegroundColor Green
}

if (Test-Path "api/proto/task/v1/generated") {
    Remove-Item -Path "api/proto/task/v1/generated" -Recurse -Force
    Write-Host "  Cleaned api/proto/task/v1/generated" -ForegroundColor Green
}

# Step 2: Create generated directories
Write-Host ""
Write-Host "Creating generated directories..." -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path "ent/generated" | Out-Null
New-Item -ItemType Directory -Force -Path "api/proto/auth/v1/generated" | Out-Null
New-Item -ItemType Directory -Force -Path "api/proto/task/v1/generated" | Out-Null
Write-Host "  Directories created" -ForegroundColor Green

# Step 3: Generate Ent code
Write-Host ""
Write-Host "Generating Ent ORM code..." -ForegroundColor Cyan
try {
    go generate ./ent
    Write-Host "  Ent code generated successfully" -ForegroundColor Green
} catch {
    Write-Host "  Failed to generate Ent code: $_" -ForegroundColor Red
    exit 1
}

# Step 4: Generate Auth service protobuf code
Write-Host ""
Write-Host "Generating Auth service protobuf code..." -ForegroundColor Cyan
try {
    $protoFile = "api/proto/auth/v1/auth.proto"
    protoc --go_out=api/proto/auth/v1/generated `
           --go_opt=paths=source_relative `
           --go-grpc_out=api/proto/auth/v1/generated `
           --go-grpc_opt=paths=source_relative `
           --proto_path=api/proto/auth/v1 `
           auth.proto
    Write-Host "  Auth protobuf generated successfully" -ForegroundColor Green
} catch {
    Write-Host "  Failed to generate Auth protobuf: $_" -ForegroundColor Red
    exit 1
}

# Step 5: Generate Task service protobuf code
Write-Host ""
Write-Host "Generating Task service protobuf code..." -ForegroundColor Cyan
try {
    $protoFile = "api/proto/task/v1/task.proto"
    protoc --go_out=api/proto/task/v1/generated `
           --go_opt=paths=source_relative `
           --go-grpc_out=api/proto/task/v1/generated `
           --go-grpc_opt=paths=source_relative `
           --proto_path=api/proto/task/v1 `
           task.proto
    Write-Host "  Task protobuf generated successfully" -ForegroundColor Green
} catch {
    Write-Host "  Failed to generate Task protobuf: $_" -ForegroundColor Red
    exit 1
}

# Step 6: Run go mod tidy
Write-Host ""
Write-Host "Updating Go dependencies..." -ForegroundColor Cyan
go mod tidy
Write-Host "  Dependencies updated" -ForegroundColor Green

# Step 7: Build to verify everything compiles
Write-Host ""
Write-Host "Building server to verify..." -ForegroundColor Cyan
$buildSuccess = $true
try {
    go build -o tmp/test-server.exe ./cmd/server
    if (Test-Path "tmp/test-server.exe") {
        Remove-Item "tmp/test-server.exe"
        Write-Host "  Build successful!" -ForegroundColor Green
    }
} catch {
    Write-Host "  Build failed: $_" -ForegroundColor Red
    $buildSuccess = $false
}

# Final summary
Write-Host ""
Write-Host "=" * 60 -ForegroundColor Cyan
if ($buildSuccess) {
    Write-Host "✅ Regeneration completed successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Yellow
    Write-Host "1. Start Docker services: docker-compose up -d" -ForegroundColor White
    Write-Host "2. Run the server: go run cmd/server/main.go" -ForegroundColor White
    Write-Host "3. Test with client: go run cmd/client/auth.go" -ForegroundColor White
} else {
    Write-Host "❌ Regeneration completed with errors" -ForegroundColor Red
    Write-Host ""
    Write-Host "Please check the errors above and fix any issues." -ForegroundColor Yellow
    Write-Host "Common issues:" -ForegroundColor Yellow
    Write-Host "- Missing protoc compiler" -ForegroundColor White
    Write-Host "- Missing Go tools (run: go install entgo.io/ent/cmd/ent@latest)" -ForegroundColor White
    Write-Host "- Import errors in service files" -ForegroundColor White
}
Write-Host "=" * 60 -ForegroundColor Cyan