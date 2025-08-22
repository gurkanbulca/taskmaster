# generate.ps1 - Generate all code into generated directories

Write-Host "Generating all code into 'generated' directories..." -ForegroundColor Yellow

# Create generated directories if they don't exist
New-Item -ItemType Directory -Force -Path "ent/generated" | Out-Null
New-Item -ItemType Directory -Force -Path "api/proto/task/v1/generated" | Out-Null

# Generate Ent code
Write-Host "Generating Ent code to ent/generated/..." -ForegroundColor Cyan
go generate ./ent

# Generate Protobuf code
Write-Host "Generating Protobuf code to api/proto/task/v1/generated/..." -ForegroundColor Cyan
protoc --go_out=api/proto/task/v1/generated --go_opt=paths=source_relative `
       --go-grpc_out=api/proto/task/v1/generated --go-grpc_opt=paths=source_relative `
       --proto_path=api/proto/task/v1 `
       task.proto

Write-Host "Code generation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Generated files structure:" -ForegroundColor Yellow
Write-Host "  ent/generated/         - Ent ORM code" -ForegroundColor Cyan
Write-Host "  api/proto/.../generated/ - Protobuf code" -ForegroundColor Cyan