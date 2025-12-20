<img src="https://raw.githubusercontent.com/motixo/goat-api/refs/heads/main/assets/mascot.png" align="right" width="250" alt="GOAT Mascot">

# GOAT API

<p>
  <a href="#features"><img src="https://img.shields.io/badge/Go-1.25-blue?logo=go" alt="Go Version"></a>
  <a href="#security"><img src="https://img.shields.io/badge/security-JWT%20%2B%20Redis-green" alt="Security"></a>
  <a href="#architecture"><img src="https://img.shields.io/badge/architecture-Clean%20Architecture-orange" alt="Architecture"></a>
  <a href="#observability"><img src="https://img.shields.io/badge/observability-Prometheus%20%2B%20Grafana-blue" alt="Observability"></a>
  <a href="https://github.com/motixo/goat-api/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-purple" alt="License"></a>
</p>

GOAT API is a **production-ready**, **secure**, and **scalable** backend application built with Go, following Clean Architecture principles. It provides a robust foundation for modern web applications with:

- 🔐 **Secure authentication** with JWT and refresh tokens
- ⚡ **High-performance session management** with Redis
- 🛡️ **Fine-grained permission system** with role-based access control
- ✋ **Advanced Rate Limiting** with Sliding Window Log algorithm
- 📊 **Comprehensive observability** with Prometheus metrics
- 🧪 **Testable architecture** with dependency injection

<br clear="right" />

## ✨ Features

### Core Architecture
- **Clean Architecture**: Strict separation of concerns with testable layers
- **Domain-Driven Design**: Rich domain model with value objects and entities
- **Repository Pattern**: Abstract data access with caching layer
- **Dependency Injection**: Compile-time DI with Google Wire

### Rate Limiting
- **Redis Sliding Window Strategy**: Prevent brute-force attacks and resource abuse.
- **Auth Limits**: Tight constraints on login/signup endpoints
- **Global Limits**: Configurable per-IP or per-User throttling via middleware


### Authentication & Security
- **JWT Authentication**: Access tokens + Refresh tokens
- **Session Management**: Real-time session tracking and revocation through Redis blacklisting and JTI rotation.
- **Argon2id Password Hashing**: With configurable pepper for enhanced security
- **ULID Generation**: Lexicographically sortable unique identifiers
- **RBAC**: Fine-grained permissions (e.g., `user:read`, `user:delete`) managed via middleware.

### Performance & Observability
- **Redis Caching**: Permission and user data caching for reduced database load
- **Atomic Operations**: Powered by Redis Lua scripts to prevent race conditions
- **Prometheus Metrics**: Comprehensive monitoring of HTTP, DB, cache, and business operations
- **Structured Logging**: Zap logger with contextual information
- **Graceful Shutdown**: Proper signal handling for zero-downtime deployments

### Developer Experience
- **Docker Support**: Containerized deployment with multi-stage builds
- **Makefile Automation**: One-command build, test, and run
- **Code Quality**: Linting and static analysis ready


## 📐 Architecture

The project follows the **Dependency Rule**: source code dependencies only point inwards.
- **Domain**: Pure business entities and interfaces (Zero external dependencies).
- **Use Cases**: Application-specific business rules and orchestrators.
- **Infrastructure**: Low-level implementations (PostgreSQL, Redis, Zap, Prometheus).
- **Delivery**: Entry points for the application (Gin HTTP, Middleware).
- **Pkg**: Shared, domain-agnostic utilities (Logger interfaces, ID generators).


## 📋 Prerequisites

- **Go 1.25+**
- **PostgreSQL 12+**
- **Redis 6+**
- **Docker**

## 🌐 API Endpoints

### Authentication
| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/auth/login` | User login with email/password |
| `POST` | `/api/v1/auth/signup` | User registration |
| `POST` | `/api/v1/auth/refresh` | Refresh access token |
| `POST` | `/api/v1/auth/logout` | Revoke current session |

### User Management
| Method | Endpoint | Description | Permissions |
|--------|----------|-------------|-------------|
| `GET` | `/api/v1/user` | Get current user profile | Authenticated |
| `GET` | `/api/v1/user/:id` | Get user by ID | `user:read` |
| `GET` | `/api/v1/user/list` | List users with filtering | `user:read` |
| `POST` | `/api/v1/user` | Create new user | `user:write` |
| `PUT` | `/api/v1/user/:id` | Update user | `user:update` |
| `PATCH` | `/api/v1/user/change-email` | Update own email | Authenticated |
| `PATCH` | `/api/v1/user/change-password` | Update own password | Authenticated |
| `PATCH` | `/api/v1/user/:id/change-role` | Update user role | `user:change_role` |
| `PATCH` | `/api/v1/user/:id/change-status` | Update user status | `user:change_status` |
| `DELETE` | `/api/v1/user` | Delete own account | Authenticated |
| `DELETE` | `/api/v1/user/:id` | Delete user | `user:delete` |

### Session Management
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/sessions` | List all active sessions |
| `DELETE` | `/api/v1/sessions` | Revoke sessions |

### Permission Management
| Method | Endpoint | Description | Permissions |
|--------|----------|-------------|-------------|
| `GET` | `/api/v1/permission` | List all permissions | `full_access` |
| `GET` | `/api/v1/permission/:role` | Get permissions by role | `full_access` |
| `POST` | `/api/v1/permission` | Create new permission | `full_access` |
| `DELETE` | `/api/v1/permission/:id` | Delete permission | `full_access` |

### Infrastructure Endpoints
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `api/health` | Health check endpoint |
| `GET` | `api/metrics` | Prometheus metrics endpoint |

## 🛠️ Development

### Project Structure
```text
├── cmd/app/                # Entry point & Wire DI configuration
├── internal/
│   ├── config/             # Environment-based configuration (envconfig)
│   ├── delivery/http/      # Handlers, Middleware, and Gin Routes
│   ├── domain/             # Entities, Value Objects, and Repository Interfaces
│   ├── usecase/            # Application business logic (Auth, User, Permission)
│   ├── infra/              # Implementation of DB, Redis, and External Services
│   ├── pkg/                # Cross-cutting concerns (Logger, IDGen, Redis Helpers)
│   └── di/                 # Google Wire Provider Sets
└── build/bin/              # Compiled binaries
```

### Quick Start
```bash
# 1. Clone the repository
git clone https://github.com/motixo/goat-api.git
cd goat-api

# 2. Copy and configure environment
cp .env.example .env
# Edit .env with your configuration

# 3. Build and run
make run

# 4. Run tests
make test

# 5. Build Docker image
make docker-build
```

### Available Make Commands
```bash
make build          # Build the application
make run            # Build and run with .env
make test           # Run all tests
make wire           # Generate Wire bindings
make clean          # Clean build artifacts
make docker-build   # Build Docker image
make help           # Show all commands
```
### Docker Deployment

```bash
# Build Image
docker build -t goat-api .

# Run Container
docker run -p 8080:8080 \
  --env-file .env \
  --name goat-api \
  goat-api
```

## 📊 Observability

- **HTTP Metrics**: Request duration, active requests, total requests by status
- **Logging**: Structured JSON logging powered by Zap.



## Support

If you encounter any issues or have questions file an issue [GitHub Issues](https://github.com/motixo/goat-api/issues)


Released under the MIT License.
