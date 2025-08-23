# scripts/setup-proto-submodule.ps1
# Setup proto submodule in backend - FIXED VERSION

$ErrorActionPreference = "Stop"

Write-Host "🔧 Setting up proto submodule for backend..." -ForegroundColor Yellow

# Check if submodule already exists
$protoGitPath = Join-Path $PSScriptRoot "..\proto\.git"
if (!(Test-Path $protoGitPath)) {
    # Add proto submodule
    Write-Host "Adding proto submodule..." -ForegroundColor Cyan
    git submodule add https://github.com/gurkanbulca/taskmaster-proto.git proto
    git submodule update --init --recursive
    Write-Host "✅ Proto submodule added" -ForegroundColor Green
} else {
    Write-Host "ℹ️  Proto submodule already exists" -ForegroundColor Cyan
    Write-Host "Updating submodule..." -ForegroundColor Cyan
    git submodule update --init --recursive
}

# Update .gitignore if needed
$gitignorePath = Join-Path $PSScriptRoot "..\.gitignore"
$gitignoreContent = Get-Content $gitignorePath -ErrorAction SilentlyContinue

$linesToAdd = @(
    "",
    "# Proto generated files",
    "api/proto/*/v1/generated/",
    "",
    "# Proto submodule generated files",
    "proto/gen/"
)

$needsUpdate = $false
foreach ($line in $linesToAdd) {
    if ($gitignoreContent -notcontains $line) {
        $needsUpdate = $true
        break
    }
}

if ($needsUpdate) {
    Write-Host "Updating .gitignore..." -ForegroundColor Cyan
    Add-Content -Path $gitignorePath -Value ($linesToAdd -join "`n")
}

Write-Host "✅ Backend proto submodule setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Run '.\build.ps1 proto' to generate Go code" -ForegroundColor White
Write-Host "2. Run '.\build.ps1 update-proto' to update proto definitions" -ForegroundColor White
Write-Host "3. Commit the submodule: git add . && git commit -m 'Add proto submodule'" -ForegroundColor White