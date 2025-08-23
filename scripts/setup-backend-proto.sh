#!/bin/bash
# setup-backend-proto.sh - Setup proto submodule in backend

set -e

echo "🔧 Setting up proto submodule for backend..."

# Add proto submodule
git submodule add https://github.com/gurkanbulca/taskmaster-proto.git proto
git submodule update --init --recursive

# Create generation script
cat > scripts/generate-from-proto.sh << 'EOF'
#!/bin/bash
# Generate Go code from proto submodule

set -e

echo "🔄 Generating Go code from proto submodule..."

# Update submodule to latest
git submodule update --remote proto

# Clean old generated code
rm -rf api/proto/*/v1/generated

# Create directories
mkdir -p api/proto/auth/v1/generated
mkdir -p api/proto/task/v1/generated
mkdir -p api/proto/common/v1/generated

# Generate Go code from submodule
cd proto
make generate-go
cd ..

# Copy generated Go code to backend structure
cp -r proto/gen/go/auth/v1/* api/proto/auth/v1/generated/
cp -r proto/gen/go/task/v1/* api/proto/task/v1/generated/
cp -r proto/gen/go/common/v1/* api/proto/common/v1/generated/

echo "✅ Go code generated from proto submodule!"
EOF

chmod +x scripts/generate-from-proto.sh

# Update Makefile
cat > Makefile << 'EOF'
.PHONY: help proto update-proto generate run test build

help:
	@echo "Available commands:"
	@echo "  make proto        - Generate Go code from proto submodule"
	@echo "  make update-proto - Update proto submodule to latest"
	@echo "  make generate     - Generate all code (Ent + Proto)"
	@echo "  make run          - Run the server"
	@echo "  make test         - Run tests"
	@echo "  make build        - Build the server"

proto:
	@./scripts/generate-from-proto.sh

update-proto:
	@echo "Updating proto submodule..."
	@git submodule update --remote proto
	@cd proto && git pull origin main
	@echo "Proto submodule updated!"

generate: proto
	@echo "Generating Ent code..."
	@go generate ./ent
	@echo "All code generated!"

run:
	go run cmd/server/main.go

test:
	go test -v -race ./...

build:
	go build -o bin/server cmd/server/main.go

docker-build:
	docker build -t taskmaster-backend .

docker-run:
	docker-compose up -d
EOF

# Update .gitignore
cat >> .gitignore << 'EOF'

# Proto generated files
api/proto/*/v1/generated/

# Proto submodule generated files
proto/gen/
EOF

# Create go.mod replace directive for local development
cat > go.mod.local << 'EOF'
// Add this to go.mod for local proto development
replace github.com/gurkanbulca/taskmaster-proto => ./proto
EOF

echo "✅ Backend proto submodule setup complete!"
echo ""
echo "Next steps:"
echo "1. Run 'make proto' to generate Go code"
echo "2. Run 'make update-proto' to update proto definitions"
echo "3. Commit the submodule: git add . && git commit -m 'Add proto submodule'"