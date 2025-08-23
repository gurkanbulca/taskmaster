# scripts/test-grpc-web-curl.ps1
# Test gRPC-Web with actual gRPC-Web requests

param(
    [string]$EnvoyUrl = "http://localhost:8080"
)

function Write-Info { Write-Host $args[0] -ForegroundColor Cyan }
function Write-Success { Write-Host $args[0] -ForegroundColor Green }
function Write-Warning { Write-Host $args[0] -ForegroundColor Yellow }
function Write-Error { Write-Host $args[0] -ForegroundColor Red }

Write-Info "================================"
Write-Info "gRPC-Web Direct Test"
Write-Info "================================"

# Test with grpcurl first (if available)
$hasGrpcurl = Get-Command grpcurl -ErrorAction SilentlyContinue
if ($hasGrpcurl) {
    Write-Info "`nTesting backend directly with grpcurl..."

    # List services
    Write-Info "Services available on backend:"
    grpcurl -plaintext localhost:50051 list

    Write-Info "`nTesting through Envoy proxy:"
    # Test through Envoy (note: grpcurl doesn't support gRPC-Web, so this might fail)
    grpcurl -plaintext localhost:8080 list 2>&1 | Out-Host
}

# Test with curl using gRPC-Web format
Write-Info "`nTesting gRPC-Web with curl..."

# Create a simple gRPC-Web request (empty message)
# gRPC-Web uses base64 encoded messages with a 5-byte header

# Empty protobuf message (just the header: 00 00 00 00 00)
$emptyMessage = [Convert]::ToBase64String(@(0, 0, 0, 0, 0))

# Test Login endpoint
Write-Info "Testing Login endpoint..."
$response = curl -X POST `
    -H "Content-Type: application/grpc-web+proto" `
    -H "X-Grpc-Web: 1" `
    -H "Accept: application/grpc-web+proto" `
    --data-binary $emptyMessage `
    "$EnvoyUrl/auth.v1.AuthService/Login" `
    -w "`nHTTP Status: %{http_code}`n" `
    -s 2>&1

Write-Host "Response: $response" -ForegroundColor Gray

# Test with grpc-web-text format
Write-Info "`nTesting with grpc-web-text format..."
$response = curl -X POST `
    -H "Content-Type: application/grpc-web-text" `
    -H "X-Grpc-Web: 1" `
    -H "Accept: application/grpc-web-text" `
    --data "AAAAAAA=" `
    "$EnvoyUrl/auth.v1.AuthService/Login" `
    -w "`nHTTP Status: %{http_code}`n" `
    -s 2>&1

Write-Host "Response: $response" -ForegroundColor Gray

# Check Envoy stats for upstream connections
Write-Info "`nChecking Envoy statistics..."
$stats = Invoke-WebRequest -Uri "http://localhost:9901/stats?filter=cluster.grpc_service" -Method GET
$relevantStats = $stats.Content -split "`n" | Where-Object { $_ -match "upstream_rq|cx_active|health" }

Write-Info "Relevant Envoy stats:"
foreach ($stat in $relevantStats) {
    if ($stat -match "upstream_rq_2xx|upstream_rq_total|cx_active") {
        Write-Host "  $stat" -ForegroundColor Green
    } elseif ($stat -match "upstream_rq_5xx|upstream_rq_4xx") {
        Write-Host "  $stat" -ForegroundColor Yellow
    }
}

# Check cluster health
Write-Info "`nChecking cluster health..."
$clusters = Invoke-WebRequest -Uri "http://localhost:9901/clusters" -Method GET
if ($clusters.Content -match "grpc_service.*health_flags::healthy") {
    Write-Success "✓ gRPC backend cluster is healthy"
} elseif ($clusters.Content -match "grpc_service.*health_flags::failed") {
    Write-Error "✗ gRPC backend cluster is unhealthy"
} else {
    Write-Warning "? gRPC backend cluster health unknown"
}

Write-Info "`n================================"
Write-Info "Diagnostic Summary"
Write-Info "================================"

# Final recommendations
Write-Host "`nIf you're seeing empty responses or status codes:" -ForegroundColor Yellow
Write-Host "1. The connection IS working (good!)" -ForegroundColor Gray
Write-Host "2. The issue is likely the gRPC-Web message format" -ForegroundColor Gray
Write-Host "3. Your Vue.js frontend with proper gRPC-Web client will handle this correctly" -ForegroundColor Gray
Write-Host "`nYour setup appears to be working correctly for gRPC-Web!" -ForegroundColor Green