#Requires -Version 5.1

<#
.SYNOPSIS
    TaskMaster Test Runner - Comprehensive testing script for Go projects
.DESCRIPTION
    Runs various types of tests for the TaskMaster gRPC project with coverage reporting,
    race detection, and detailed output formatting.
.PARAMETER TestType
    Type of tests to run: unit, integration, e2e, or all
.PARAMETER Coverage
    Generate coverage report
.PARAMETER VerboseOutput
    Enable verbose test output
.PARAMETER Race
    Enable race detection
.PARAMETER Short
    Run tests in short mode
.PARAMETER Timeout
    Test timeout in minutes (default: 10)
.PARAMETER Package
    Specific package to test (optional)
.PARAMETER CoverageThreshold
    Minimum coverage percentage required (default: 70)
.PARAMETER OutputFormat
    Test output format: standard, json, or junit
.PARAMETER Clean
    Clean test cache and coverage files before running
.EXAMPLE
    .\run_tests.ps1 -TestType all -Coverage
.EXAMPLE
    .\run_tests.ps1 -TestType unit -Package "./internal/service" -Verbose
.EXAMPLE
    .\run_tests.ps1 -TestType integration -Race -Timeout 15
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory = $false)]
    [ValidateSet("unit", "integration", "e2e", "all")]
    [string]$TestType = "unit",

    [Parameter(Mandatory = $false)]
    [switch]$Coverage = $false,

    [Parameter(Mandatory = $false)]
    [switch]$VerboseOutput = $false,

    [Parameter(Mandatory = $false)]
    [switch]$Race = $false,

    [Parameter(Mandatory = $false)]
    [switch]$Short = $false,

    [Parameter(Mandatory = $false)]
    [ValidateRange(1, 60)]
    [int]$Timeout = 10,

    [Parameter(Mandatory = $false)]
    [string]$Package = "",

    [Parameter(Mandatory = $false)]
    [ValidateRange(0, 100)]
    [int]$CoverageThreshold = 70,

    [Parameter(Mandatory = $false)]
    [ValidateSet("standard", "json", "junit")]
    [string]$OutputFormat = "standard",

    [Parameter(Mandatory = $false)]
    [switch]$Clean = $false,

    [Parameter(Mandatory = $false)]
    [switch]$Help = $false
)

# Script configuration
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

# Constants
$SCRIPT_NAME = "TaskMaster Test Runner"
$SCRIPT_VERSION = "2.0.0"
$COVERAGE_DIR = "coverage"
$COVERAGE_FILE = "$COVERAGE_DIR/coverage.out"
$COVERAGE_HTML = "$COVERAGE_DIR/coverage.html"
$COVERAGE_XML = "$COVERAGE_DIR/coverage.xml"
$TEST_RESULTS_DIR = "test-results"

# Colors for output
$Colors = @{
    Header    = "Cyan"
    Success   = "Green"
    Warning   = "Yellow"
    Error     = "Red"
    Info      = "White"
    Secondary = "Gray"
}

function Write-ColoredOutput {
    param(
        [string]$Message,
        [string]$Color = "White",
        [switch]$NoNewline = $false
    )

    if ($NoNewline) {
        Write-Host $Message -ForegroundColor $Color -NoNewline
    } else {
        Write-Host $Message -ForegroundColor $Color
    }
}

function Write-Header {
    param([string]$Title)

    Write-Host ""
    Write-ColoredOutput "=" * 60 -Color $Colors.Header
    Write-ColoredOutput "  $Title" -Color $Colors.Header
    Write-ColoredOutput "=" * 60 -Color $Colors.Header
    Write-Host ""
}

function Write-Section {
    param([string]$Title)

    Write-Host ""
    Write-ColoredOutput "🔹 $Title" -Color $Colors.Info
    Write-ColoredOutput "-" * 40 -Color $Colors.Secondary
}

function Show-Help {
    Write-Header "$SCRIPT_NAME v$SCRIPT_VERSION - Help"

    Write-ColoredOutput "DESCRIPTION:" -Color $Colors.Info
    Write-ColoredOutput "  Comprehensive test runner for TaskMaster gRPC project" -Color $Colors.Secondary
    Write-Host ""

    Write-ColoredOutput "USAGE:" -Color $Colors.Info
    Write-ColoredOutput "  .\run_tests.ps1 [OPTIONS]" -Color $Colors.Secondary
    Write-Host ""

    Write-ColoredOutput "OPTIONS:" -Color $Colors.Info
    Write-ColoredOutput "  -TestType <type>         Test type: unit, integration, e2e, or all (default: unit)" -Color $Colors.Secondary
    Write-ColoredOutput "  -Coverage               Generate coverage report" -Color $Colors.Secondary
    Write-ColoredOutput "  -VerboseOutput          Enable verbose test output" -Color $Colors.Secondary
    Write-ColoredOutput "  -Race                   Enable race detection" -Color $Colors.Secondary
    Write-ColoredOutput "  -Short                  Run tests in short mode" -Color $Colors.Secondary
    Write-ColoredOutput "  -Timeout <minutes>      Test timeout in minutes (default: 10)" -Color $Colors.Secondary
    Write-ColoredOutput "  -Package <path>         Specific package to test" -Color $Colors.Secondary
    Write-ColoredOutput "  -CoverageThreshold <n>  Minimum coverage percentage (default: 70)" -Color $Colors.Secondary
    Write-ColoredOutput "  -OutputFormat <format>  Output format: standard, json, junit (default: standard)" -Color $Colors.Secondary
    Write-ColoredOutput "  -Clean                  Clean test cache and coverage files" -Color $Colors.Secondary
    Write-ColoredOutput "  -Help                   Show this help message" -Color $Colors.Secondary
    Write-Host ""

    Write-ColoredOutput "EXAMPLES:" -Color $Colors.Info
    Write-ColoredOutput "  .\run_tests.ps1 -TestType all -Coverage" -Color $Colors.Secondary
    Write-ColoredOutput "  .\run_tests.ps1 -TestType unit -Package `"./internal/service`" -VerboseOutput" -Color $Colors.Secondary
    Write-ColoredOutput "  .\run_tests.ps1 -TestType integration -Race -Timeout 15" -Color $Colors.Secondary
    Write-ColoredOutput "  .\run_tests.ps1 -Clean -Coverage -CoverageThreshold 80" -Color $Colors.Secondary
    Write-Host ""
}

function Test-Prerequisites {
    Write-Section "Checking Prerequisites"

    # Check if Go is installed
    try {
        $goVersion = go version 2>$null
        if ($goVersion) {
            Write-ColoredOutput "✅ Go found: $goVersion" -Color $Colors.Success
        } else {
            throw "Go not found"
        }
    } catch {
        Write-ColoredOutput "❌ Go is not installed or not in PATH" -Color $Colors.Error
        exit 1
    }

    # Check if we're in a Go project
    if (-not (Test-Path "go.mod")) {
        Write-ColoredOutput "❌ go.mod not found. Are you in a Go project directory?" -Color $Colors.Error
        exit 1
    }

    Write-ColoredOutput "✅ Go project detected" -Color $Colors.Success

    # Check if generated code exists
    $generatedPaths = @(
        "ent/generated",
        "api/proto/auth/v1/generated",
        "api/proto/task/v1/generated"
    )

    $missingGenerated = @()
    foreach ($path in $generatedPaths) {
        if (-not (Test-Path $path)) {
            $missingGenerated += $path
        }
    }

    if ($missingGenerated.Count -gt 0) {
        Write-ColoredOutput "⚠️ Missing generated code:" -Color $Colors.Warning
        foreach ($path in $missingGenerated) {
            Write-ColoredOutput "   - $path" -Color $Colors.Warning
        }
        Write-ColoredOutput "   Run .\generate.ps1 to generate missing code" -Color $Colors.Warning
        Write-Host ""
    } else {
        Write-ColoredOutput "✅ Generated code found" -Color $Colors.Success
    }
}

function Initialize-Directories {
    Write-Section "Initializing Directories"

    # Create coverage directory
    if (-not (Test-Path $COVERAGE_DIR)) {
        New-Item -ItemType Directory -Path $COVERAGE_DIR -Force | Out-Null
        Write-ColoredOutput "✅ Created coverage directory" -Color $Colors.Success
    }

    # Create test results directory
    if (-not (Test-Path $TEST_RESULTS_DIR)) {
        New-Item -ItemType Directory -Path $TEST_RESULTS_DIR -Force | Out-Null
        Write-ColoredOutput "✅ Created test results directory" -Color $Colors.Success
    }
}

function Clean-TestCache {
    Write-Section "Cleaning Test Cache"

    try {
        # Clean Go test cache
        go clean -testcache 2>$null
        Write-ColoredOutput "✅ Cleaned Go test cache" -Color $Colors.Success

        # Remove coverage files
        if (Test-Path $COVERAGE_DIR) {
            Remove-Item -Path "$COVERAGE_DIR/*" -Recurse -Force -ErrorAction SilentlyContinue
            Write-ColoredOutput "✅ Cleaned coverage files" -Color $Colors.Success
        }

        # Remove test result files
        if (Test-Path $TEST_RESULTS_DIR) {
            Remove-Item -Path "$TEST_RESULTS_DIR/*" -Recurse -Force -ErrorAction SilentlyContinue
            Write-ColoredOutput "✅ Cleaned test result files" -Color $Colors.Success
        }

    } catch {
        Write-ColoredOutput "⚠️ Failed to clean some cache files: $($_.Exception.Message)" -Color $Colors.Warning
    }
}

function Build-GoTestCommand {
    param(
        [string]$TestPattern = "",
        [string]$PackagePath = "./..."
    )

    $args = @("test")

    # Add package path
    $args += $PackagePath

    # Add test pattern if specified
    if ($TestPattern) {
        $args += "-run", $TestPattern
    }

    # Add verbosity
    if ($VerboseOutput) {
        $args += "-v"
    }

    # Add race detection
    if ($Race) {
        $args += "-race"
    }

    # Add short mode
    if ($Short) {
        $args += "-short"
    }

    # Add timeout
    $args += "-timeout", "${Timeout}m"

    # Add coverage
    if ($Coverage) {
        $args += "-coverprofile=$COVERAGE_FILE"
        $args += "-covermode=atomic"
    }

    # Add output format
    switch ($OutputFormat) {
        "json" { $args += "-json" }
        "junit" {
            # Note: Go doesn't natively support JUnit, would need additional tooling
            Write-ColoredOutput "⚠️ JUnit output requires additional tooling (go-junit-report)" -Color $Colors.Warning
        }
    }

    return $args
}

function Run-Tests {
    param(
        [string]$Type,
        [string]$PackagePath = "./..."
    )

    Write-Section "Running $Type Tests"

    # Display test configuration
    Write-ColoredOutput "Test Configuration:" -Color $Colors.Info
    Write-ColoredOutput "  • Type: $Type" -Color $Colors.Secondary
    Write-ColoredOutput "  • Package: $PackagePath" -Color $Colors.Secondary
    Write-ColoredOutput "  • Coverage: $Coverage" -Color $Colors.Secondary
    Write-ColoredOutput "  • Verbose: $VerboseOutput" -Color $Colors.Secondary
    Write-ColoredOutput "  • Race Detection: $Race" -Color $Colors.Secondary
    Write-ColoredOutput "  • Short Mode: $Short" -Color $Colors.Secondary
    Write-ColoredOutput "  • Timeout: ${Timeout}m" -Color $Colors.Secondary
    Write-ColoredOutput "  • Output Format: $OutputFormat" -Color $Colors.Secondary
    Write-Host ""

    # Build test pattern based on type
    $testPattern = switch ($Type) {
        "unit" { "^Test[^_]*$|^TestUnit" }
        "integration" { "^TestIntegration" }
        "e2e" { "^TestE2E|^TestEndToEnd" }
        "all" { "" }
        default { "" }
    }

    # Build and execute command
    $testArgs = Build-GoTestCommand -TestPattern $testPattern -PackagePath $PackagePath

    Write-ColoredOutput "Executing: go $($testArgs -join ' ')" -Color $Colors.Info
    Write-Host ""

    try {
        $startTime = Get-Date

        # Execute tests
        & go @testArgs
        $exitCode = $LASTEXITCODE

        $endTime = Get-Date
        $duration = $endTime - $startTime

        Write-Host ""
        if ($exitCode -eq 0) {
            Write-ColoredOutput "✅ Tests completed successfully in $($duration.TotalSeconds.ToString('F2')) seconds" -Color $Colors.Success
        } else {
            Write-ColoredOutput "❌ Tests failed with exit code $exitCode after $($duration.TotalSeconds.ToString('F2')) seconds" -Color $Colors.Error
            return $false
        }

    } catch {
        Write-ColoredOutput "❌ Failed to run tests: $($_.Exception.Message)" -Color $Colors.Error
        return $false
    }

    return $true
}

function Generate-CoverageReport {
    if (-not $Coverage -or -not (Test-Path $COVERAGE_FILE)) {
        return
    }

    Write-Section "Generating Coverage Report"

    try {
        # Generate HTML report
        go tool cover -html=$COVERAGE_FILE -o $COVERAGE_HTML
        Write-ColoredOutput "✅ HTML coverage report: $COVERAGE_HTML" -Color $Colors.Success

        # Calculate coverage percentage
        $coverageOutput = go tool cover -func=$COVERAGE_FILE | Select-String "total:"
        if ($coverageOutput) {
            $coverageMatch = $coverageOutput -match "(\d+\.\d+)%"
            if ($coverageMatch) {
                $coveragePercent = [double]$matches[1]

                Write-Host ""
                Write-ColoredOutput "📊 Coverage Summary:" -Color $Colors.Info
                Write-ColoredOutput "  • Total Coverage: $($coveragePercent)%" -Color $Colors.Secondary
                Write-ColoredOutput "  • Threshold: $CoverageThreshold%" -Color $Colors.Secondary

                if ($coveragePercent -ge $CoverageThreshold) {
                    Write-ColoredOutput "  • Status: ✅ PASSED" -Color $Colors.Success
                } else {
                    Write-ColoredOutput "  • Status: ❌ FAILED (below threshold)" -Color $Colors.Error
                    Write-ColoredOutput "  • Improvement needed: $($CoverageThreshold - $coveragePercent)%" -Color $Colors.Warning
                    return $false
                }
            }
        }

    } catch {
        Write-ColoredOutput "⚠️ Failed to generate coverage report: $($_.Exception.Message)" -Color $Colors.Warning
    }

    return $true
}

function Show-TestSummary {
    param([bool]$TestsSuccess, [bool]$CoverageSuccess = $true)

    Write-Header "Test Summary"

    if ($TestsSuccess -and $CoverageSuccess) {
        Write-ColoredOutput "🎉 All tests and coverage checks PASSED!" -Color $Colors.Success
        Write-Host ""
        Write-ColoredOutput "Next Steps:" -Color $Colors.Info
        Write-ColoredOutput "  • Review coverage report: $COVERAGE_HTML" -Color $Colors.Secondary
        Write-ColoredOutput "  • Consider adding more tests for uncovered code" -Color $Colors.Secondary
        Write-ColoredOutput "  • Run integration tests if you haven't: .\run_tests.ps1 -TestType integration" -Color $Colors.Secondary
    } else {
        Write-ColoredOutput "❌ Some tests or coverage checks FAILED!" -Color $Colors.Error
        Write-Host ""
        Write-ColoredOutput "Troubleshooting:" -Color $Colors.Info

        if (-not $TestsSuccess) {
            Write-ColoredOutput "  • Review test failures above" -Color $Colors.Secondary
            Write-ColoredOutput "  • Check for missing dependencies or generated code" -Color $Colors.Secondary
            Write-ColoredOutput "  • Run .\generate.ps1 if needed" -Color $Colors.Secondary
        }

        if (-not $CoverageSuccess) {
            Write-ColoredOutput "  • Add more unit tests to improve coverage" -Color $Colors.Secondary
            Write-ColoredOutput "  • Focus on uncovered functions and branches" -Color $Colors.Secondary
            Write-ColoredOutput "  • Consider lowering threshold temporarily: -CoverageThreshold 60" -Color $Colors.Secondary
        }

        exit 1
    }
}

# Main execution
function Main {
    try {
        # Show help if requested
        if ($Help) {
            Show-Help
            exit 0
        }

        Write-Header "$SCRIPT_NAME v$SCRIPT_VERSION"

        # Prerequisites check
        Test-Prerequisites

        # Initialize directories
        Initialize-Directories

        # Clean if requested
        if ($Clean) {
            Clean-TestCache
        }

        # Determine packages to test
        $packagePath = if ($Package) { $Package } else { "./..." }

        # Run tests based on type
        $testsSuccess = switch ($TestType) {
            "all" {
                $unitSuccess = Run-Tests -Type "unit" -PackagePath $packagePath
                $integrationSuccess = Run-Tests -Type "integration" -PackagePath $packagePath
                $e2eSuccess = Run-Tests -Type "e2e" -PackagePath $packagePath
                $unitSuccess -and $integrationSuccess -and $e2eSuccess
            }
            default {
                Run-Tests -Type $TestType -PackagePath $packagePath
            }
        }

        # Generate coverage report
        $coverageSuccess = Generate-CoverageReport

        # Show summary
        Show-TestSummary -TestsSuccess $testsSuccess -CoverageSuccess $coverageSuccess

    } catch {
        Write-ColoredOutput "❌ Unexpected error: $($_.Exception.Message)" -Color $Colors.Error
        Write-ColoredOutput "Stack trace: $($_.ScriptStackTrace)" -Color $Colors.Error
        exit 1
    }
}

# Execute main function
Main