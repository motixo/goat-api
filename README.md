<img src="https://raw.githubusercontent.com/motixo/goat-api/refs/heads/main/assets/mascot.png" align="right" width="250" alt="GOAT Mascot">

# GOAT API

<p>
  <a href="#features"><img src="https://img.shields.io/badge/Go-1.21%2B-blue?logo=go" alt="Go Version"></a>
  <a href="#security"><img src="https://img.shields.io/badge/security-JWT%20%2B%20Redis-green" alt="Security"></a>
  <a href="#architecture"><img src="https://img.shields.io/badge/architecture-Clean%20Architecture-orange" alt="Architecture"></a>
  <a href="#observability"><img src="https://img.shields.io/badge/observability-Prometheus%20%2B%20Grafana-blue" alt="Observability"></a>
  <a href="https://github.com/motixo/goat-api/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-purple" alt="License"></a>
</p>

GOAT API is a **production-ready**, **secure**, and **scalable** backend application built with Go, following Clean Architecture principles. It provides a robust foundation for modern web applications with:

- 🔐 **Secure authentication** with JWT and refresh tokens
- ⚡ **High-performance session management** with Redis
- 🛡️ **Fine-grained permission system** with role-based access control
- 📊 **Comprehensive observability** with Prometheus metrics
- 🧪 **Testable architecture** with dependency injection

<br clear="right" />

## ✨ Features

### Core Architecture
- **Clean Architecture**: Strict separation of concerns with testable layers
- **Domain-Driven Design**: Rich domain model with value objects and entities
- **Repository Pattern**: Abstract data access with caching layer
- **Dependency Injection**: Compile-time DI with Google Wire

### Authentication & Security
- **JWT Authentication**: Access tokens (15m) + Refresh tokens (7 days)
- **Redis Session Management**: Real-time session tracking and revocation
- **Argon2id Password Hashing**: With configurable pepper for enhanced security
- **ULID Generation**: Lexicographically sortable unique identifiers
- **Role-Based Access Control (RBAC)**: Fine-grained permissions system

### Performance & Observability
- **Redis Caching**: Permission and user data caching for reduced database load
- **Prometheus Metrics**: Comprehensive monitoring of HTTP, DB, cache, and business operations
- **Structured Logging**: Zap logger with contextual information
- **Graceful Shutdown**: Proper signal handling for zero-downtime deployments

### Developer Experience
- **Docker Support**: Containerized deployment with multi-stage builds
- **Makefile Automation**: One-command build, test, and run
- **Comprehensive Testing**: Unit tests with 100% domain coverage
- **Code Quality**: Linting and static analysis ready

## 📐 Architecture

```
┌─────────────────┐
│   Delivery      │  Gin HTTP Handlers, Middleware, Routes
├─────────────────┤
│   Use Cases     │  Application Business Logic
├─────────────────┤
│     Domain      │  Entities, Value Objects, Domain Services
├─────────────────┤
│   infra         │  PostgreSQL, Redis, JWT, Logger, Metrics
└─────────────────┘
```

### Key Design Principles
- **Dependency Rule**: Source code dependencies only point inward
- **Framework Independence**: Domain layer has zero external dependencies
- **Testability**: All layers can be unit tested in isolation
- **Scalability**: Stateless design with external session storage

## 📋 Prerequisites

- **Go 1.21+**
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
```
├── cmd/
│   └── app/                    # Application entry point
├── internal/
│   ├── config/                 # Configuration management
│   ├── delivery/http/          # HTTP layer (handlers, middleware, routes)
│   ├── domain/                 # Core business logic
│   │   ├── entity/            # Domain entities
│   │   ├── valueobject/       # Value objects (UserRole, UserStatus, etc.)
│   │   ├── service/           # Domain service interfaces
│   │   ├── repository/        # Repository interfaces
│   │   ├── usecase/           # Use cases with business rules
│   │   └── validation/        # Validation logic
│   ├── infra/                  # Infrastructure implementations
│   │   ├── auth/              # JWT and password services
│   │   ├── cache/             # Redis cache implementations
│   │   ├── database/postgres/ # PostgreSQL repositories
│   │   ├── logger/            # Zap logger implementation
│   │   ├── metrics/           # Prometheus metrics
│   │   └── storage/redis/     # Redis session storage
│   └── di/                     # Dependency injection providers
├── assets/                     # Static assets (mascot, etc.)
├── Makefile                    # Development automation
└── go.mod                      # Go module definition
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

## 🔒 Security

### Authentication Flow
1. **Login**: User provides credentials → receives access + refresh tokens
2. **Access**: Access token used for API requests (15-minute expiry)
3. **Refresh**: Refresh token used to obtain new access token (7-day expiry)
4. **Logout**: Session revoked from Redis, tokens invalidated

### Security Features
- **Password Security**: Argon2id hashing with configurable time/memory costs
- **Session Management**: Redis-based sessions with real-time revocation
- **Input Validation**: Comprehensive validation at HTTP and domain layers
- **Error Handling**: Generic error messages to prevent information leakage
- **Rate Limiting**: Built-in (configurable) rate limiting for authentication endpoints
- **HTTPS Ready**: Production configuration supports TLS termination

### Permission System
- **Role Hierarchy**: Admin → Operator → Client
- **Granular Permissions**: Fine-grained control over user operations
- **Self vs Admin Operations**: Separate permissions for self-modification vs admin operations
- **Automatic Filtering**: Users can only see data they have permission to access

## 📊 Observability

### Metrics Exposed
- **HTTP Metrics**: Request duration, active requests, total requests by status
- **Database Metrics**: Query duration and success rate by operation
- **Cache Metrics**: Hit/miss ratios for user and permission caches
- **Business Metrics**: Login attempts, token refreshes, user registrations

### Monitoring Setup
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'goat-api'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

## 🐳 Docker Deployment

### Build Image
```bash
docker build -t goat-api .
```

### Run Container
```bash
docker run -p 8080:8080 \
  --env-file .env \
  --name goat-api \
  goat-api
```

## Support

If you encounter any issues or have questions:

- **File an issue**: [GitHub Issues](https://github.com/motixo/goat-api/issues)
- **Documentation**: This README contains comprehensive setup and usage information
