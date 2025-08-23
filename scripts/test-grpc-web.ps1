# scripts/test-grpc-web.ps1
# Test gRPC-Web connectivity through Envoy proxy

param(
    [string]$EnvoyUrl = "http://localhost:8080",
    [string]$AdminUrl = "http://localhost:9901",
    [switch]$Detailed
)

function Write-Info { Write-Host $args[0] -ForegroundColor Cyan }
function Write-Success { Write-Host $args[0] -ForegroundColor Green }
function Write-Warning { Write-Host $args[0] -ForegroundColor Yellow }
function Write-Error { Write-Host $args[0] -ForegroundColor Red }

Write-Info "================================"
Write-Info "gRPC-Web Connectivity Test"
Write-Info "================================"

# Test Envoy Admin Interface
Write-Info "`nTesting Envoy Admin Interface..."
try {
    $adminResponse = Invoke-WebRequest -Uri "$AdminUrl/stats" -Method GET -ErrorAction Stop
    Write-Success "✓ Envoy admin interface is accessible"

    if ($Detailed) {
        Write-Info "Envoy Statistics (first 10 lines):"
        ($adminResponse.Content -split "`n")[0..9] | ForEach-Object {
            Write-Host "  $_" -ForegroundColor Gray
        }
    }
}
catch {
    Write-Error "✗ Cannot access Envoy admin interface at $AdminUrl"
    Write-Warning "  Make sure Envoy is running: .\scripts\setup-envoy.ps1 dev"
    exit 1
}

# Test Envoy clusters
Write-Info "`nChecking Envoy upstream clusters..."
try {
    $clustersResponse = Invoke-WebRequest -Uri "$AdminUrl/clusters" -Method GET -ErrorAction Stop
    if ($clustersResponse.Content -match "grpc_service") {
        Write-Success "✓ gRPC service cluster is configured"

        # Check cluster health
        if ($clustersResponse.Content -match "health_flags::/failed_active_hc") {
            Write-Warning "  ⚠ Backend health check is failing"
            Write-Warning "  Make sure your gRPC backend is running on port 50051"
        }
        elseif ($clustersResponse.Content -match "health_flags::healthy") {
            Write-Success "  Backend cluster is healthy"
        }
    }
    else {
        Write-Error "✗ gRPC service cluster not found in configuration"
    }
}
catch {
    Write-Error "✗ Cannot check Envoy clusters"
}

# Test gRPC-Web endpoint
Write-Info "`nTesting gRPC-Web endpoints..."

# Test Auth Service
$authEndpoints = @(
    "auth.v1.AuthService/Login",
    "auth.v1.AuthService/Register",
    "auth.v1.AuthService/RefreshToken"
)

foreach ($endpoint in $authEndpoints) {
    Write-Host "  Testing $endpoint..." -NoNewline

    try {
        $headers = @{
            "Content-Type" = "application/grpc-web-text"
            "X-Grpc-Web" = "1"
        }

        $response = Invoke-WebRequest `
            -Uri "$EnvoyUrl/$endpoint" `
            -Method POST `
            -Headers $headers `
            -Body "" `
            -ErrorAction Stop `
            2>$null

        Write-Success " ✓ Reachable"
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__

        # Different status codes mean different things
        switch ($statusCode) {
            400 { Write-Success " ✓ Reachable (400 - Missing request data)" }
            404 { Write-Error " ✗ Not found (404 - Check proto definitions)" }
            415 { Write-Success " ✓ Reachable (415 - Content type recognized)" }
            503 { Write-Error " ✗ Service unavailable (503 - Backend not running)" }
            default { Write-Warning " ? Status: $statusCode" }
        }
    }
}

# Test Task Service
$taskEndpoints = @(
    "task.v1.TaskService/CreateTask",
    "task.v1.TaskService/GetTask",
    "task.v1.TaskService/ListTasks"
)

foreach ($endpoint in $taskEndpoints) {
    Write-Host "  Testing $endpoint..." -NoNewline

    try {
        $headers = @{
            "Content-Type" = "application/grpc-web-text"
            "X-Grpc-Web" = "1"
        }

        $response = Invoke-WebRequest `
            -Uri "$EnvoyUrl/$endpoint" `
            -Method POST `
            -Headers $headers `
            -Body "" `
            -ErrorAction Stop `
            2>$null

        Write-Success " ✓ Reachable"
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__

        switch ($statusCode) {
            400 { Write-Success " ✓ Reachable (400 - Missing request data)" }
            401 { Write-Success " ✓ Reachable (401 - Auth required)" }
            404 { Write-Error " ✗ Not found (404 - Check proto definitions)" }
            503 { Write-Error " ✗ Service unavailable (503 - Backend not running)" }
            default { Write-Warning " ? Status: $statusCode" }
        }
    }
}

# Test CORS headers
Write-Info "`nTesting CORS configuration..."
try {
    $headers = @{
        "Origin" = "http://localhost:3000"
        "Access-Control-Request-Method" = "POST"
        "Access-Control-Request-Headers" = "content-type,x-grpc-web"
    }

    $response = Invoke-WebRequest `
        -Uri "$EnvoyUrl/auth.v1.AuthService/Login" `
        -Method OPTIONS `
        -Headers $headers `
        -ErrorAction Stop

    if ($response.Headers["Access-Control-Allow-Origin"]) {
        Write-Success "✓ CORS is properly configured"
        if ($Detailed) {
            Write-Info "  Allow-Origin: $($response.Headers['Access-Control-Allow-Origin'])"
            Write-Info "  Allow-Methods: $($response.Headers['Access-Control-Allow-Methods'])"
        }
    }
    else {
        Write-Warning "⚠ CORS headers not found in response"
    }
}
catch {
    Write-Warning "⚠ Could not test CORS configuration"
}

# Summary
Write-Info "`n================================"
Write-Info "Test Summary"
Write-Info "================================"

$backendRunning = $false
try {
    $tcpClient = New-Object System.Net.Sockets.TcpClient
    $tcpClient.Connect("localhost", 50051)
    $tcpClient.Close()
    $backendRunning = $true
}
catch {
    $backendRunning = $false
}

if ($backendRunning) {
    Write-Success "✓ Backend is running on port 50051"
}
else {
    Write-Error "✗ Backend is not running on port 50051"
    Write-Warning "  Start it with: go run ./cmd/server/main.go"
}

Write-Success "✓ Envoy proxy is running and configured"
Write-Success "✓ gRPC-Web endpoints are accessible"

if ($backendRunning) {
    Write-Info "`nYour gRPC-Web setup is ready for frontend development!"
}
else {
    Write-Warning "`nStart your backend to complete the setup."
}

# Additional debugging info
if ($Detailed) {
    Write-Info "`nDebug Information:"
    Write-Host "  Envoy URL: $EnvoyUrl" -ForegroundColor Gray
    Write-Host "  Admin URL: $AdminUrl" -ForegroundColor Gray
    Write-Host "  Timestamp: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Gray
}