# TaskMaster - Production-Ready gRPC Server with Go

A high-performance, scalable gRPC task management service built with Go, featuring Ent ORM, PostgreSQL, and production-ready patterns.

## 🚀 Features

- **gRPC API** with Protocol Buffers for efficient communication
- **Ent ORM** for type-safe database operations and automatic migrations
- **PostgreSQL** database with connection pooling
- **Clean Architecture** with repository pattern
- **Hot Reload** development with Air
- **Docker Compose** for local development
- **Health Checks** and service reflection
- **Structured Logging** and error handling
- **Ready for** authentication, caching, and observability

## 📋 Prerequisites

- Go 1.21+
- Protocol Buffers compiler (protoc)
- Docker & Docker Compose
- Windows/Linux/macOS

## 🛠️ Quick Setup

### 1. Clone and Navigate
```bash
git clone https://github.com/gurkanbulca/taskmaster.git
cd taskmaster
```

### 2. Install Dependencies

#### Windows (PowerShell)
```powershell
# Install tools
go install entgo.io/ent/cmd/ent@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/cosmtrek/air@latest

# Run setup script
.\setup-with-ent.ps1
```

#### Linux/macOS
```bash
# Install tools
go install entgo.io/ent/cmd/ent@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/cosmtrek/air@latest

# Run setup
make setup
```

### 3. Configure Environment
```bash
cp .env.example .env
# Edit .env with your settings
```

### 4. Start Services
```bash
# Start PostgreSQL, Redis, Jaeger
docker-compose up -d

# Run the server
go run cmd/server/main.go
```

## 🏗️ Project Structure

```
taskmaster/
├── api/
│   └── proto/          # Protocol buffer definitions
│       └── task/v1/
│           └── task.proto
├── cmd/
│   ├── server/         # Main server application
│   │   └── main.go
│   ├── client/         # Test client
│   │   └── main.go
│   └── migrate/        # Migration tool
│       └── main.go
├── ent/
│   ├── schema/         # Ent schema definitions
│   │   └── task.go
│   └── ...            # Generated Ent code
├── internal/
│   ├── config/         # Configuration management
│   ├── database/       # Database connection
│   ├── repository/     # Data access layer
│   ├── service/        # Business logic
│   └── middleware/     # gRPC interceptors
├── pkg/                # Reusable packages
├── scripts/            # Utility scripts
├── deployments/        # Deployment configs
│   ├── docker/
│   └── k8s/
├── .env.example        # Environment template
├── docker-compose.yml  # Local development
├── go.mod
└── README.md
```

## 🔧 Development

### Essential Commands

```bash
# Generate Ent code after schema changes
go generate ./ent

# Generate protobuf code
make proto
# or
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/task/v1/task.proto

# Run with hot reload
air

# Run tests
go test ./...

# Build binary
go build -o bin/server cmd/server/main.go

# Run migrations manually
go run cmd/migrate/main.go
```

### Windows Commands (PowerShell)
```powershell
# Use the build script
.\build.ps1 help     # Show available commands
.\build.ps1 proto    # Generate protobuf
.\build.ps1 build    # Build binary
.\build.ps1 run      # Run server
.\build.ps1 test     # Run tests
```

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
- ID (UUID)
- Title (string, required)
- Description (text)
- Status (enum: pending, in_progress, completed, cancelled)
- Priority (enum: low, medium, high, critical)
- AssignedTo (string, optional)
- DueDate (timestamp, optional)
- Tags ([]string)
- Metadata (JSON)
- CreatedAt (timestamp)
- UpdatedAt (timestamp)
```

### Modifying Schema
1. Edit `ent/schema/task.go`
2. Run `go generate ./ent`
3. Restart server (auto-migrates)

## 🐳 Docker Services

```yaml
# PostgreSQL - Database
localhost:5432

# Redis - Caching (ready for implementation)
localhost:6379

# Jaeger - Distributed tracing (ready for implementation)
localhost:16686 (UI)
```

## 🚀 Production Deployment

### Building for Production
```bash
# Build Docker image
docker build -t taskmaster:latest .

# Run with Docker
docker run -p 50051:50051 taskmaster:latest
```

### Environment Variables
```env
# Server
GRPC_PORT=50051
HTTP_PORT=8080
ENVIRONMENT=production

# Database
DB_HOST=your-db-host
DB_PORT=5432
DB_USER=your-db-user
DB_PASSWORD=your-db-password
DB_NAME=taskmaster
DB_SSL_MODE=require

# Redis (for future implementation)
REDIS_HOST=your-redis-host
REDIS_PORT=6379
```

## 📈 Performance

- Connection pooling with configurable limits
- Efficient Ent ORM queries with eager loading
- Indexed database columns for fast queries
- Ready for caching layer implementation

## 🔐 Security (To Be Implemented)

- [ ] JWT authentication
- [ ] mTLS for service-to-service communication
- [ ] Rate limiting
- [ ] API key management
- [ ] RBAC (Role-Based Access Control)

## 🔍 Observability (To Be Implemented)

- [ ] Prometheus metrics
- [ ] Jaeger distributed tracing
- [ ] Structured logging with zerolog
- [ ] Health checks ✅ (implemented)
- [ ] Custom dashboards

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run integration tests
go test ./test/integration/...

# Load testing with k6 (to be implemented)
k6 run scripts/k6/load_test.js
```

## 📚 Learning Resources

### Technologies Used
- [gRPC-Go](https://grpc.io/docs/languages/go/)
- [Ent ORM](https://entgo.io/docs/getting-started/)
- [Protocol Buffers](https://protobuf.dev/programming-guides/proto3/)
- [PostgreSQL](https://www.postgresql.org/docs/)

### Design Patterns
- Repository Pattern
- Clean Architecture
- SOLID Principles
- Domain-Driven Design (DDD)

## 🛣️ Roadmap

### Phase 1 (Current) ✅
- [x] Basic gRPC server setup
- [x] Ent ORM integration
- [x] CRUD operations
- [x] PostgreSQL database
- [x] Docker Compose setup

### Phase 2 (In Progress)
- [ ] JWT authentication
- [ ] User management
- [ ] Request validation
- [ ] Error handling improvements

### Phase 3 (Planned)
- [ ] Redis caching
- [ ] Prometheus metrics
- [ ] Jaeger tracing
- [ ] Rate limiting

### Phase 4 (Future)
- [ ] Kubernetes deployment
- [ ] CI/CD pipeline
- [ ] GraphQL gateway
- [ ] Event sourcing

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 👨‍💻 Author

**Gurkan Bulca**
- GitHub: [@gurkanbulca](https://github.com/gurkanbulca)

## 🙏 Acknowledgments

- Anthropic Claude for development assistance
- Ent team for the amazing ORM
- gRPC team for the framework
- Go community for excellent tools and libraries

---

**Happy Coding! 🚀**