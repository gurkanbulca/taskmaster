# TaskMaster - Production-Ready gRPC Server with Go

A high-performance, scalable gRPC task management service built with Go, featuring Ent ORM, PostgreSQL, and production-ready patterns.

## 🚀 Features

- **gRPC API** with Protocol Buffers for efficient communication
- **Ent ORM** for type-safe database operations and automatic migrations
- **PostgreSQL** database with connection pooling
- **Clean Architecture** with repository pattern
- **Generated Code Separation** - Clean distinction between source and generated files
- **Hot Reload** development with Air
- **Docker Compose** for local development
- **Health Checks** and service reflection
- **Structured Logging** and error handling
- **Ready for** authentication, caching, and observability

## 📋 Prerequisites

- Go 1.21+
- Protocol Buffers compiler (protoc)
- Docker & Docker Compose
- Git

## 🛠️ Quick Setup

### 1. Clone the Repository
```bash
git clone https://github.com/gurkanbulca/taskmaster.git
cd taskmaster
```

### 2. Install Required Tools

#### Windows (PowerShell)
```powershell
# Install tools
go install entgo.io/ent/cmd/ent@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/cosmtrek/air@latest

# Install protoc via Scoop
scoop install protobuf
```

#### Linux/macOS
```bash
# Install tools
go install entgo.io/ent/cmd/ent@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/cosmtrek/air@latest

# Install protoc
# macOS
brew install protobuf

# Linux
sudo apt install -y protobuf-compiler
```

### 3. Generate Code
**Important**: Generated code is not included in the repository. You must generate it after cloning.

```bash
# Windows
.\generate.ps1

# Linux/macOS
chmod +x generate.sh
./generate.sh
```

### 4. Configure Environment
```bash
cp .env.example .env
# Edit .env with your settings (optional, defaults work for local development)
```

### 5. Start Services
```bash
# Start PostgreSQL, Redis, and Jaeger
docker-compose up -d

# Run the server
go run cmd/server/main.go

# Or use hot reload
air
```

## 🏗️ Project Structure

```
taskmaster/
├── api/
│   └── proto/
│       └── task/v1/
│           ├── task.proto          # ✅ Source (tracked in Git)
│           └── generated/          # ❌ Generated (not in Git)
│               ├── task.pb.go
│               └── task_grpc.pb.go
├── ent/
│   ├── schema/
│   │   └── task.go                # ✅ Source (tracked in Git)
│   ├── generate.go                # ✅ Source (tracked in Git)
│   └── generated/                 # ❌ Generated (not in Git)
│       ├── client.go
│       ├── task.go
│       └── ...
├── cmd/
│   ├── server/                    # Main server application
│   ├── client/                    # Test client
│   └── migrate/                   # Migration tool
├── internal/
│   ├── config/                    # Configuration management
│   ├── database/                  # Database connection
│   ├── repository/                # Data access layer
│   ├── service/                   # Business logic
│   └── middleware/                # gRPC interceptors
├── scripts/                       # Utility scripts
├── deployments/                   # Deployment configs
├── .env.example                   # Environment template
├── .gitignore                     # Git ignore rules
├── docker-compose.yml             # Local services
├── generate.ps1                   # Windows code generation
├── generate.sh                    # Linux/macOS code generation
├── go.mod
└── README.md
```

### Source vs Generated Files

| Type | Location | In Git? | Description |
|------|----------|---------|-------------|
| **Source** | `ent/schema/*.go` | ✅ Yes | Ent schema definitions |
| **Source** | `api/proto/**/*.proto` | ✅ Yes | Protocol buffer definitions |
| **Source** | `internal/**/*.go` | ✅ Yes | Business logic |
| **Generated** | `**/generated/` | ❌ No | All generated code |
| **Generated** | `*.pb.go` | ❌ No | Protobuf Go code |
| **Config** | `.env` | ❌ No | Local configuration |
| **Config** | `.env.example` | ✅ Yes | Configuration template |

## 🔧 Development

### Essential Commands

```bash
# Generate all code (Ent + Protobuf)
# Windows
.\generate.ps1

# Linux/macOS
./generate.sh

# Run with hot reload
air

# Run tests
go test ./...

# Build binary
go build -o bin/server cmd/server/main.go

# Run migrations (automatic on server start)
go run cmd/migrate/main.go

# Clean generated files
# Windows
Remove-Item -Recurse -Force ent/generated, api/proto/task/v1/generated

# Linux/macOS
rm -rf ent/generated api/proto/task/v1/generated
```

### Modifying Schemas

#### Update Ent Schema
1. Edit `ent/schema/task.go`
2. Run `.\generate.ps1` or `./generate.sh`
3. Restart server (migrations run automatically)

#### Update Proto Definitions
1. Edit `api/proto/task/v1/task.proto`
2. Run `.\generate.ps1` or `./generate.sh`
3. Update service implementations if needed

## 📡 API Endpoints

### gRPC Services

#### TaskService
- `CreateTask` - Create a new task
- `GetTask` - Get task by ID
- `ListTasks` - List tasks with filtering
- `UpdateTask` - Update existing task
- `DeleteTask` - Delete a task
- `WatchTasks` - Stream task events (server-streaming)

### Testing the API

#### Using grpcurl
```bash
# List available services
grpcurl -plaintext localhost:50051 list

# Describe service
grpcurl -plaintext localhost:50051 describe task.v1.TaskService

# Create a task
grpcurl -plaintext -d '{
  "title": "Complete project",
  "description": "Finish the gRPC implementation",
  "priority": "PRIORITY_HIGH"
}' localhost:50051 task.v1.TaskService/CreateTask

# List tasks
grpcurl -plaintext -d '{"page_size": 10}' \
  localhost:50051 task.v1.TaskService/ListTasks
```

#### Using the Test Client
```bash
go run cmd/client/main.go
```

## 🗄️ Database Schema (Ent)

### Task Entity
```go
Fields:
- ID (UUID, auto-generated)
- Title (string, required)
- Description (text, optional)
- Status (enum: pending, in_progress, completed, cancelled)
- Priority (enum: low, medium, high, critical)
- AssignedTo (string, optional)
- DueDate (timestamp, optional)
- Tags ([]string)
- Metadata (JSON)
- CreatedAt (timestamp, auto)
- UpdatedAt (timestamp, auto)

Indexes:
- status
- priority
- assigned_to
- status + priority (composite)
- created_at
```

## 🐳 Docker Services

```yaml
PostgreSQL: localhost:5432
Redis:      localhost:6379 (ready for caching)
Jaeger:     localhost:16686 (UI for tracing)
```

### Managing Services
```bash
# Start all services
docker-compose up -d

# Stop all services
docker-compose down

# View logs
docker-compose logs -f postgres

# Reset database
docker-compose down -v
docker-compose up -d
```

## 🚀 Production Deployment

### Building for Production
```bash
# Build Docker image
docker build -t taskmaster:latest .

# Run with Docker
docker run -p 50051:50051 \
  -e DB_HOST=your-db-host \
  -e DB_PASSWORD=your-password \
  taskmaster:latest
```

### Environment Variables
See `.env.example` for all available configuration options.

Key variables:
- `GRPC_PORT` - gRPC server port (default: 50051)
- `DB_*` - PostgreSQL connection settings
- `REDIS_*` - Redis connection settings (for future use)
- `JWT_SECRET` - JWT signing key (for future auth)
- `ENVIRONMENT` - development/staging/production

## 📈 Performance

- **Connection Pooling**: Configurable database connection limits
- **Efficient Queries**: Ent ORM generates optimized SQL
- **Indexed Columns**: Fast queries on common filters
- **Prepared for Caching**: Redis integration ready

## 🔐 Security (Roadmap)

- [ ] JWT authentication
- [ ] Rate limiting per client
- [ ] mTLS for service communication
- [ ] API key management
- [ ] RBAC (Role-Based Access Control)

## 🔍 Observability (Roadmap)

- [x] Health checks
- [ ] Prometheus metrics
- [ ] Jaeger distributed tracing
- [ ] Structured logging with zerolog
- [ ] Custom Grafana dashboards

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run specific package tests
go test ./internal/service/...

# Run with verbose output
go test -v ./...
```

## 🛣️ Roadmap

### Phase 1 - Core (Completed ✅)
- [x] gRPC server setup
- [x] Ent ORM integration
- [x] CRUD operations
- [x] PostgreSQL database
- [x] Docker Compose setup
- [x] Clean code organization

### Phase 2 - Enhancement (Current)
- [ ] JWT authentication
- [ ] User management
- [ ] Request validation middleware
- [ ] Enhanced error handling
- [ ] Unit tests

### Phase 3 - Scalability
- [ ] Redis caching layer
- [ ] Prometheus metrics
- [ ] Jaeger tracing
- [ ] Rate limiting
- [ ] Circuit breaker

### Phase 4 - Production
- [ ] Kubernetes manifests
- [ ] Helm charts
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] API Gateway
- [ ] GraphQL layer

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Generate code after schema changes (`./generate.ps1` or `./generate.sh`)
4. Commit only source files (generated files are git-ignored)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

### Development Guidelines
- Never commit generated code (`**/generated/` directories)
- Always update `.proto` or schema files, not generated code
- Run `go fmt` before committing
- Add tests for new features
- Update documentation as needed

## 📝 Common Issues & Solutions

### Generated code not found
```bash
# Solution: Generate the code
./generate.ps1  # Windows
./generate.sh   # Linux/macOS
```

### PostgreSQL connection refused
```bash
# Solution: Start Docker services
docker-compose up -d
```

### Import errors after pulling updates
```bash
# Solution: Regenerate code
./generate.ps1  # or ./generate.sh
```

## 📚 Learning Resources

- [gRPC-Go Documentation](https://grpc.io/docs/languages/go/)
- [Ent ORM Documentation](https://entgo.io/docs/getting-started/)
- [Protocol Buffers Guide](https://protobuf.dev/programming-guides/proto3/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 👨‍💻 Author

**Gurkan Bulca**
- GitHub: [@gurkanbulca](https://github.com/gurkanbulca)

## 🙏 Acknowledgments

- Anthropic Claude for development assistance
- Ent team for the excellent ORM
- gRPC team for the framework
- Go community for amazing tools

---

**Happy Coding! 🚀**