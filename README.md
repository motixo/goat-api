<p align="left">
  <img src="https://raw.githubusercontent.com/motixo/goat-api/refs/heads/main/assets/mascot.png" alt="GOAT Mascot" width="250">
</p>

# GOAT API - under develop

A production-grade Go backend built on Clean Architecture, featuring JWT authentication with refresh tokens, Redis-powered session management and permission caching, and PostgreSQL for data persistence—all designed for security, performance, and maintainability.

## Table of Contents
- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [API Endpoints](#api-endpoints)
- [Development](#development)
- [Security](#security)

## Overview

Goat API is a production-ready backend application built with Go, following Clean Architecture principles. It provides a secure, scalable foundation for web applications with modern authentication, session management, and permission-based access control.

## Features

- **Clean Architecture**: Well-structured, testable, and maintainable codebase
- **JWT Authentication**: Secure token-based authentication with refresh tokens
- **Redis Sessions**: Session management with Redis for high performance
- **PostgreSQL**: Robust database persistence
- **Permission System**: Role-based access control (RBAC)
- **ULID Generation**: Universally unique lexicographically sortable identifiers
- **Password Security**: Argon2id hashing with pepper
- **Caching**: Permission caching for improved performance
- **Dependency Injection**: Google Wire for compile-time dependency injection
- **Gin Framework**: High-performance HTTP framework
- **Structured Logging**: Zap logger integration
- **Domain-Driven Design**: Event-driven architecture support

## Architecture

This project follows Clean Architecture principles:

```
┌─────────────────┐
│   Delivery      │  HTTP Handlers, Middleware
├─────────────────┤
│ Infrastructure  │  PostgreSQL, Redis, JWT, Logger
├─────────────────┤
│     Domain      │  Entities, Value Objects, Use Cases, Services
├─────────────────┤
│   Interfaces    │  Repository & Service Contracts
└─────────────────┘
```

### Key Components

- **Domain Layer**: Business logic, entities, and value objects
- **Use Cases**: Business rules and application logic
- **Infrastructure**: Database, cache, external services
- **Delivery**: HTTP handlers and middleware
- **Repository Pattern**: Abstract data access with caching layer

## Prerequisites

- Go 1.21+
- PostgreSQL 12+
- Redis 6+
- Docker (optional, for containerized deployment)


## API Endpoints

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/auth/login` | User login |
| `POST` | `/auth/signup` | User registration |
| `POST` | `/auth/refresh` | Token refresh |
| `POST` | `/auth/logout` | User logout |

### User Management

| Method | Endpoint | Description | Permissions |
|--------|----------|-------------|-------------|
| `GET` | `/user/` | Get current user info | Authenticated |
| `GET` | `/user/:id` | Get user by ID | `user:read` |
| `GET` | `/user/list` | Get all users | `user:read` |
| `PUT` | `/user/change-password` | Update own password | Authenticated |
| `PUT` | `/user/change-role` | Update user role | `user:change_role` |
| `PUT` | `/user/change-status` | Update user status | `user:change_status` |
| `DELETE` | `/user/delete` | Delete own account | Authenticated |
| `DELETE` | `/user/delete/:id` | Delete user by ID | `user:delete` |


### Session Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/sessions` | List user sessions |
| `DELETE` | `/sessions` | Revoke session |

## Development

### Project Structure

```
├── cmd/
│   └── app/
│       ├── main.go           # Application entry point
│       └── wire.go          # Wire dependency injection

├── internal/
│   ├── config/             # Configuration management
│   ├── delivery/           # HTTP handlers and middleware
│   │   └── http/
│   ├── domain/             # Business logic
│   │   ├── entity/         # Domain entities
│   │   ├── valueobject/    # Value objects
│   │   ├── service/        # Domain services
│   │   ├── repository/     # Repository interfaces
│   │   ├── usecase/        # Use cases
│   │   └── validation/     # Validation logic
│   ├── infrastructure/     # Infrastructure implementations
│   │   ├── auth/           # JWT and auth services
│   │   ├── cache/          # Cache implementations
│   │   ├── database/       # Database implementations
│   │   ├── logger/         # Logger implementations
│   │   └── storage/        # Storage implementations
│   ├── test/              # Test files
│   └── wire/              # Wire provider sets
└── go.mod                 # Go module definition
```


## Security

### Authentication & Authorization

- JWT tokens with 15-minute access token TTL
- Refresh tokens with 7-day expiration
- Redis-based session management
- Permission-based access control
- Argon2id password hashing with pepper


## Support

If you encounter any issues or have questions, please file an issue on the [GitHub repository](https://github.com/motixo/goat-api/issues).

---
