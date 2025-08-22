# 🚀 TaskMaster - Production-Ready gRPC Task Management System

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/doc/go1.24)
[![License](https://img.shields.io/badge/license-MIT-blue?style=for-the-badge)](LICENSE)
[![gRPC](https://img.shields.io/badge/gRPC-v1.75-244c5a?style=for-the-badge&logo=grpc)](https://grpc.io/)
[![Ent](https://img.shields.io/badge/Ent-v0.14.5-5b9bd5?style=for-the-badge)](https://entgo.io/)

A high-performance, enterprise-grade task management system built with Go, featuring comprehensive security, email verification, password reset, and audit logging capabilities.

## ✨ Features

### 🔐 Phase 2 Security Features (Complete)
- **Email Verification System**: Token-based email verification with configurable expiry
- **Password Reset**: Secure password reset flow with rate limiting
- **Account Lockout**: Configurable failed login attempts and lockout duration
- **Security Event Logging**: Complete audit trail with severity levels
- **Rate Limiting**: Protect against brute force and abuse
- **Session Management**: JWT with refresh tokens and configurable timeout

### 📋 Core Functionality
- **Task Management**: Full CRUD operations with creator/assignee relationships
- **User Management**: Role-based access control (User/Manager/Admin)
- **Real-time Updates**: Server-streaming gRPC for live task events
- **Relationship Management**: Task hierarchies with parent/subtask support

### 🛡️ Security & Authentication
- **JWT Authentication**: Dual token system (access + refresh)
- **Password Security**: bcrypt hashing with configurable complexity requirements
- **Email Notifications**: SMTP/Mock email service for security alerts
- **Security Event Tracking**: Login attempts, password changes, suspicious activity
- **Account Protection**: Automatic lockout, password history, session invalidation

### 🏗️ Technical Architecture
- **gRPC API**: High-performance RPC with Protocol Buffers
- **Ent ORM**: Type-safe database operations with automatic migrations
- **PostgreSQL**: Primary database with optimized indexes and connection pooling
- **Clean Architecture**: Repository pattern, service layer, middleware chain
- **Code Generation**: Separate generated code for clean version control
- **Hot Reload**: Development with Air for rapid iteration
- **Docker Compose**: Complete local development environment