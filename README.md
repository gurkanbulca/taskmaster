# TaskMaster - Production-Ready gRPC Server with Go

A high-performance, scalable gRPC task management service built with Go, featuring comprehensive authentication, Ent ORM, PostgreSQL, and production-ready patterns.

## 🚀 Features

### Core Services
- **🔐 Authentication Service** - JWT-based auth with user management
- **📋 Task Management Service** - Full CRUD operations with relations
- **👥 User Management** - Role-based access control (User, Manager, Admin)
- **🔗 Task-User Relations** - Creator and assignee relationships

### Technical Stack
- **gRPC API** with Protocol Buffers for efficient communication
- **Ent ORM** for type-safe database operations and automatic migrations
- **PostgreSQL** database with connection pooling and indexes
- **JWT Authentication** with access/refresh token pattern
- **bcrypt Password Hashing** with configurable requirements
- **Clean Architecture** with repository pattern and middleware
- **Generated Code Separation** - Clean distinction between source and generated files
- **Hot Reload** development with Air
- **Docker Compose** for local development
- **Health Checks** and service reflection
- **Structured Logging** and error handling

### Security & Permissions
- **Role-based Authorization** (User/Manager/Admin)
- **Task Ownership** - Users can only access their created/assigned tasks
- **Protected Endpoints** with middleware-based authentication
- **Password Security** with bcrypt and validation
- **Token Management** with secure refresh patterns

## 📋 Prerequisites

- Go 1.24+
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
│       ├── auth/v1/
│       │   ├── auth.proto          # ✅ Auth service definitions
│       │   └── generated/          # ❌ Generated protobuf code
│       └── task/v1/
│           ├── task.proto          # ✅ Task service definitions
│           └── generated/          # ❌ Generated protobuf code
├── ent/
│   ├── schema/
│   │   ├── user.go                # ✅ User entity schema
│   │   └── task.go                # ✅ Task entity schema
│   ├── generate.go                # ✅ Ent code generation config
│   └── generated/                 # ❌ Generated Ent ORM code
├── cmd/
│   ├── server/                    # Main server application
│   ├── client/                    # Test clients (auth & task)
│   └── migrate/                   # Database migration tool
├── internal/
│   ├── config/                    # Configuration management
│   ├── database/                  # Database connection (Ent)
│   ├── repository/                # Data access layer (Ent-based)
│   ├── service/                   # Business logic (Auth & Task)
│   ├── middleware/                # gRPC interceptors (Auth & Validation)
│   └── models/                    # Legacy models (deprecated)
├── pkg/
│   └── auth/                      # JWT & password utilities
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
| **Source** | `ent/schema/*.go` | ✅ Yes | User & Task entity definitions |
| **Source** | `api/proto/**/*.proto` | ✅ Yes | Auth & Task service definitions |
| **Source** | `internal/**/*.go` | ✅ Yes | Business logic & middleware |
| **Source** | `pkg/**/*.go` | ✅ Yes | Shared utilities |
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
Remove-Item -Recurse -Force ent/generated, api/proto/*/v1/generated

# Linux/macOS
rm -rf ent/generated api/proto/*/v1/generated
```

### Modifying Schemas

#### Update Ent Schema (Database)
1. Edit `ent/schema/user.go` or `ent/schema/task.go`
2. Run `.\generate.ps1` or `./generate.sh`
3. Restart server (migrations run automatically)

#### Update Proto Definitions (API)
1. Edit `api/proto/auth/v1/auth.proto` or `api/proto/task/v1/task.proto`
2. Run `.\generate.ps1` or `./generate.sh`
3. Update service implementations if needed

## 📡 API Services

### 🔐 AuthService

#### Authentication Endpoints
- `Register` - Create new user account
- `Login` - Authenticate with email/username and password
- `RefreshToken` - Generate new access token using refresh token
- `Logout` - Invalidate refresh token

#### User Management
- `GetMe` - Get current authenticated user info
- `UpdateProfile` - Update user profile (name, preferences)
- `ChangePassword` - Change user password

#### Future Features (Placeholder)
- `VerifyEmail` - Email verification
- `RequestPasswordReset` - Initiate password reset
- `ResetPassword` - Complete password reset

### 📋 TaskService

#### Task Management
- `CreateTask` - Create a new task (auto-assigned to creator)
- `GetTask` - Get task by ID (with permission checks)
- `ListTasks` - List tasks with filtering (role-based access)
- `UpdateTask` - Update existing task (with permission checks)
- `DeleteTask` - Delete a task (creator or admin only)
- `WatchTasks` - Stream task events (server-streaming)

#### Permission Model
- **Users**: Can only see/modify tasks they created or are assigned to
- **Managers**: Can see tasks from their scope
- **Admins**: Full access to all tasks

## 🗄️ Database Schema

### User Entity (Authentication & Authorization)
```
Fields:
- ID (UUID, auto-generated)
- Email (string, unique, required)
- Username (string, unique, required)
- PasswordHash (string, sensitive)
- FirstName, LastName (optional)
- Role (enum: user, manager, admin)
- IsActive, EmailVerified (boolean)
- RefreshToken (string, sensitive)
- RefreshTokenExpiresAt (timestamp)
- Preferences (JSON)
- LastLogin (timestamp)
- CreatedAt, UpdatedAt (auto-managed)

Indexes:
- email (unique)
- username (unique)
- email + is_active (login queries)
- role + is_active (authorization)
```

### Task Entity (Task Management)
```
Fields:
- ID (UUID, auto-generated)
- Title (string, required)
- Description (text, optional)
- Status (enum: pending, in_progress, completed, cancelled)
- Priority (enum: low, medium, high, critical)
- AssignedTo (string, optional) - Email/identifier
- DueDate (timestamp, optional)
- Tags ([]string)
- Metadata (JSON)
- CreatedAt, UpdatedAt (auto-managed)

Relations:
- Creator (User) - Many tasks to one user
- Assignee (User) - Many tasks to one user (optional)
- Parent/Subtasks - Self-referencing for task hierarchy

Indexes:
- status, priority, assigned_to
- status + priority (composite)
- created_at, due_date
```

## 🧪 Testing the API

### Using the Test Clients

#### Authentication Test Client
```bash
go run cmd/client/auth.go
```
Features:
- User registration and login
- Token refresh and logout
- Profile updates and password changes
- Permission testing

#### Task Test Client
```bash
go run cmd/client/main.go
```
Features:
- Task CRUD operations
- Permission validation
- Relationship testing

### Using grpcurl

#### List Services
```bash
grpcurl -plaintext localhost:50051 list
```

#### Authentication Examples
```bash
# Register a new user
grpcurl -plaintext -d '{
  "email": "user@example.com",
  "username": "testuser",
  "password": "SecurePass123!",
  "first_name": "Test",
  "last_name": "User"
}' localhost:50051 auth.v1.AuthService/Register

# Login
grpcurl -plaintext -d '{
  "email": "user@example.com",
  "password": "SecurePass123!"
}' localhost:50051 auth.v1.AuthService/Login
```

#### Task Management Examples (with auth token)
```bash
# Create a task (requires auth header)
grpcurl -plaintext \
  -H "authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "title": "Complete project",
    "description": "Finish the gRPC implementation",
    "priority": "PRIORITY_HIGH"
  }' localhost:50051 task.v1.TaskService/CreateTask

# List tasks
grpcurl -plaintext \
  -H "authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{"page_size": 10}' \
  localhost:50051 task.v1.TaskService/ListTasks
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

## 🔐 Security Features

### Implemented
- **JWT Authentication** with access/refresh token pattern
- **bcrypt Password Hashing** with configurable strength
- **Role-based Authorization** (User/Manager/Admin)
- **Input Validation** middleware
- **Password Requirements** (length, complexity)
- **Token Expiration** and secure refresh
- **Sensitive Field Protection** (passwords, tokens not logged)

### Security Configuration
```go
// Password Requirements (configurable)
MinLength: 8 characters
RequireUpper: true
RequireLower: true  
RequireNumber: true
RequireSpecial: false (configurable)

// JWT Settings
AccessTokenDuration: 15 minutes (configurable)
RefreshTokenDuration: 7 days (configurable)
Signing Algorithm: HS256
```

## ⚡ Performance Features

- **Connection Pooling**: Configurable database connection limits
- **Efficient Queries**: Ent ORM generates optimized SQL with proper indexes
- **Lazy Loading**: Relations loaded only when needed
- **Batch Operations**: Support for bulk task operations
- **Transaction Support**: Atomic multi-operation updates
- **Prepared for Caching**: Redis integration ready

## 🚀 Production Deployment

### Building for Production
```bash
# Build Docker image
docker build -t taskmaster:latest .

# Run with Docker
docker run -p 50051:50051 \
  -e DB_HOST=your-db-host \
  -e DB_PASSWORD=your-password \
  -e JWT_ACCESS_SECRET=your-secret \
  taskmaster:latest
```

### Environment Variables
See `.env.example` for all available configuration options.

Key production variables:
- `GRPC_PORT` - gRPC server port (default: 50051)
- `DB_*` - PostgreSQL connection settings
- `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET` - **Must be changed in production**
- `JWT_ACCESS_TOKEN_DURATION`, `JWT_REFRESH_TOKEN_DURATION` - Token lifetimes
- `ENVIRONMENT` - development/staging/production

⚠️ **Security Warning**: Change all default secrets before production deployment!

## 🔍 Observability (Ready for Implementation)

- [x] Health checks (`/grpc.health.v1.Health/Check`)
- [x] gRPC reflection for development
- [x] Structured logging with request tracing
- [x] User context in logs
- [ ] Prometheus metrics
- [ ] Jaeger distributed tracing
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

### ✅ Phase 1 - Core (Completed)
- [x] gRPC server with authentication and task services
- [x] Ent ORM with User and Task entities
- [x] JWT authentication with refresh tokens
- [x] Role-based authorization
- [x] CRUD operations with permissions
- [x] PostgreSQL database with proper relations
- [x] Docker Compose setup
- [x] Comprehensive test clients

### 🔄 Phase 2 - Enhancement (Current)
- [ ] Email verification system
- [ ] Password reset functionality
- [ ] Request validation enhancements
- [ ] Comprehensive unit tests
- [ ] API documentation generation

### 🔮 Phase 3 - Scalability
- [ ] Redis caching layer
- [ ] Prometheus metrics
- [ ] Jaeger tracing
- [ ] Rate limiting
- [ ] Circuit breaker pattern

### 🚀 Phase 4 - Production
- [ ] Kubernetes manifests
- [ ] Helm charts
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] API Gateway integration
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
- Follow the existing code style and patterns

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
# Solution: Regenerate code and update dependencies
./generate.ps1  # or ./generate.sh
go mod tidy
```

### Authentication failures in tests
```bash
# Solution: Check JWT secrets in .env
# Make sure JWT_ACCESS_SECRET and JWT_REFRESH_SECRET are set
```

## 📚 Learning Resources

- [gRPC-Go Documentation](https://grpc.io/docs/languages/go/)
- [Ent ORM Documentation](https://entgo.io/docs/getting-started/)
- [Protocol Buffers Guide](https://protobuf.dev/programming-guides/proto3/)
- [JWT Best Practices](https://tools.ietf.org/html/rfc7519)
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

*Built with ❤️ using Go, gRPC, and Ent ORM*