#!/bin/bash
# setup.sh - Complete setup script for Linux/macOS (UTF-8)

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${YELLOW}$1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${CYAN}ℹ️  $1${NC}"
}

# Main setup
echo -e "${GREEN}🚀 Setting up TaskMaster with Ent ORM${NC}"
echo ""

# Step 1: Check prerequisites
print_status "Checking prerequisites..."

# Check Go
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.21+ first."
    exit 1
fi
print_success "Go is installed: $(go version)"

# Check protoc
if ! command -v protoc &> /dev/null; then
    print_error "protoc is not installed. Please install Protocol Buffers compiler."
    echo "  macOS: brew install protobuf"
    echo "  Linux: sudo apt install -y protobuf-compiler"
    exit 1
fi
print_success "protoc is installed: $(protoc --version)"

# Check Docker
if ! command -v docker &> /dev/null; then
    print_error "Docker is not installed. Please install Docker first."
    exit 1
fi
print_success "Docker is installed: $(docker --version)"

# Step 2: Install Go tools
print_status "Installing Go tools..."

# Install Ent CLI
go install entgo.io/ent/cmd/ent@latest
print_success "Installed Ent CLI"

# Install protoc plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
print_success "Installed protoc plugins"

# Install Air for hot reload
go install github.com/cosmtrek/air@latest
print_success "Installed Air for hot reload"

# Install migrate tool
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
print_success "Installed migrate tool"

# Install grpcurl
if ! command -v grpcurl &> /dev/null; then
    go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
    print_success "Installed grpcurl"
else
    print_info "grpcurl already installed"
fi

# Step 3: Install Go dependencies
print_status "Installing Go dependencies..."
go mod download
go mod tidy
print_success "Go dependencies installed"

# Step 4: Initialize Ent if needed
print_status "Setting up Ent ORM..."
if [ ! -d "ent" ]; then
    go run entgo.io/ent/cmd/ent init Task
    print_success "Initialized Ent with Task schema"
else
    print_info "Ent already initialized"
fi

# Step 5: Generate Ent code
print_status "Generating Ent code..."
go generate ./ent
print_success "Ent code generated"

# Step 6: Generate protobuf files
print_status "Generating protobuf files..."
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/task/v1/task.proto
print_success "Protobuf files generated"

# Step 7: Setup environment
print_status "Setting up environment..."
if [ ! -f .env ]; then
    cp .env.example .env
    print_success "Created .env file from .env.example"
    echo ""
    print_info "Please edit .env file with your configuration"
else
    print_info ".env file already exists"
fi

# Step 8: Start Docker services
print_status "Starting Docker services..."
docker-compose up -d
print_success "Docker services started"

# Wait for PostgreSQL to be ready
print_status "Waiting for PostgreSQL to be ready..."
sleep 5

# Check if PostgreSQL is accessible
if docker exec taskmaster-postgres-1 pg_isready -U postgres > /dev/null 2>&1; then
    print_success "PostgreSQL is ready"
else
    # Try with different container name
    if docker exec taskmaster_postgres_1 pg_isready -U postgres > /dev/null 2>&1; then
        print_success "PostgreSQL is ready"
    else
        print_error "PostgreSQL is not ready. Please check Docker logs."
    fi
fi

# Step 9: Run migrations
print_status "Running database migrations..."
if go run cmd/migrate/main.go; then
    print_success "Database migrations completed"
else
    print_info "Migrations will run when server starts"
fi

# Step 10: Build the application
print_status "Building the application..."
go build -o bin/server cmd/server/main.go
print_success "Server binary built: bin/server"

# Step 11: Create useful scripts
print_status "Creating helper scripts..."

# Create run.sh
cat > run.sh << 'EOF'
#!/bin/bash
# Run the server with environment variables
source .env
go run cmd/server/main.go
EOF
chmod +x run.sh
print_success "Created run.sh"

# Create test.sh
cat > test.sh << 'EOF'
#!/bin/bash
# Run all tests
echo "Running tests..."
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
echo "Coverage report generated: coverage.html"
EOF
chmod +x test.sh
print_success "Created test.sh"

# Print summary
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}✅ Setup completed successfully!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${CYAN}📚 Quick Start:${NC}"
echo "  1. Run the server:    ./run.sh"
echo "  2. Or with hot reload: air"
echo "  3. Test with client:   go run cmd/client/main.go"
echo ""
echo -e "${CYAN}🔧 Development Commands:${NC}"
echo "  go generate ./ent     - Regenerate Ent code"
echo "  make proto            - Regenerate protobuf"
echo "  make test             - Run tests"
echo "  make build            - Build binary"
echo ""
echo -e "${CYAN}📡 Test the API:${NC}"
echo "  grpcurl -plaintext localhost:50051 list"
echo "  grpcurl -plaintext localhost:50051 describe task.v1.TaskService"
echo ""
echo -e "${CYAN}🐳 Docker Services:${NC}"
echo "  PostgreSQL: localhost:5432"
echo "  Redis:      localhost:6379"
echo "  Jaeger UI:  http://localhost:16686"
echo ""
echo -e "${GREEN}Happy coding! 🚀${NC}"