# generate.ps1 - Generate all code into generated directories

Write-Host "Generating all code into 'generated' directories..." -ForegroundColor Yellow

# Create generated directories if they don't exist
New-Item -ItemType Directory -Force -Path "ent/generated" | Out-Null
New-Item -ItemType Directory -Force -Path "api/proto/task/v1/generated" | Out-Null
New-Item -ItemType Directory -Force -Path "api/proto/auth/v1/generated" | Out-Null

# Generate Ent code (includes User schema now)
Write-Host "Generating Ent code to ent/generated/..." -ForegroundColor Cyan
go generate ./ent

# Generate Task service protobuf code
Write-Host "Generating Task service protobuf code..." -ForegroundColor Cyan
protoc --go_out=api/proto/task/v1/generated --go_opt=paths=source_relative `
       --go-grpc_out=api/proto/task/v1/generated --go-grpc_opt=paths=source_relative `
       --proto_path=api/proto/task/v1 `
       task.proto

# Generate Auth service protobuf code
Write-Host "Generating Auth service protobuf code..." -ForegroundColor Cyan
protoc --go_out=api/proto/auth/v1/generated --go_opt=paths=source_relative `
       --go-grpc_out=api/proto/auth/v1/generated --go-grpc_opt=paths=source_relative `
       --proto_path=api/proto/auth/v1 `
       auth.proto

Write-Host "Code generation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Generated files structure:" -ForegroundColor Yellow
Write-Host "  ent/generated/           - Ent ORM code (with User & Task)" -ForegroundColor Cyan
Write-Host "  api/proto/task/v1/generated/ - Task service protobuf" -ForegroundColor Cyan
Write-Host "  api/proto/auth/v1/generated/ - Auth service protobuf" -ForegroundColor Cyan