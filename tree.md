go-core/
├── cmd/ # Application entry points
│ ├── server/
│ │ └── main.go # HTTP server entry point
│ ├── worker/
│ │ └── main.go # Background worker entry point
│ ├── migrate/
│ │ └── main.go # Database migration tool
│ └── cli/
│ └── main.go # CLI tools
├── internal/ # Private application code
│ ├── app/ # Feature modules (domain-driven)
│ │ ├── auth/
│ │ │ ├── handler.go # HTTP handlers
│ │ │ ├── service.go # Business logic
│ │ │ ├── repository.go # Data access
│ │ │ ├── models.go # Domain models
│ │ │ ├── validation.go # Input validation
│ │ │ ├── events.go # Domain events
│ │ │ └── handler_test.go
│ │ ├── users/
│ │ │ ├── handler.go
│ │ │ ├── service.go
│ │ │ ├── repository.go
│ │ │ ├── models.go
│ │ │ ├── validation.go
│ │ │ ├── events.go
│ │ │ └── handler_test.go
│ │ ├── organizations/
│ │ └── notifications/
│ ├── lib/ # Reusable infrastructure packages
│ │ ├── cache/
│ │ │ ├── cache.go # Cache interface & implementation
│ │ │ ├── memory.go # In-memory cache (sync.Map)
│ │ │ ├── redis.go # Redis client (optional)
│ │ │ └── cache_test.go
│ │ ├── queue/
│ │ │ ├── queue.go # Queue interface
│ │ │ ├── memory.go # Channel-based queue
│ │ │ ├── job.go # Job definition
│ │ │ ├── worker.go # Worker pool
│ │ │ └── queue_test.go
│ │ ├── websocket/
│ │ │ ├── hub.go # WebSocket hub/manager
│ │ │ ├── client.go # WebSocket client
│ │ │ ├── message.go # Message types
│ │ │ └── websocket_test.go
│ │ ├── database/
│ │ │ ├── connection.go # Database connection pool
│ │ │ ├── repository.go # Base repository pattern
│ │ │ ├── transaction.go # Transaction management
│ │ │ ├── migrations/
│ │ │ │ ├── migrate.go # Migration runner
│ │ │ │ └── migrations/
│ │ │ │ └── 001_initial.sql
│ │ │ └── database_test.go
│ │ ├── events/
│ │ │ ├── bus.go # Event bus (channel-based)
│ │ │ ├── event.go # Event interface
│ │ │ ├── handler.go # Event handler interface
│ │ │ └── events_test.go
│ │ ├── monitoring/
│ │ │ ├── logger.go # Structured logging (log/slog)
│ │ │ ├── metrics.go # Metrics collection
│ │ │ ├── health/
│ │ │ │ ├── checker.go # Health check interface
│ │ │ │ ├── database.go # Database health check
│ │ │ │ └── memory.go # Memory health check
│ │ │ └── monitoring_test.go
│ │ ├── storage/
│ │ │ ├── storage.go # Storage interface
│ │ │ ├── local.go # Local filesystem storage
│ │ │ ├── s3.go # S3-compatible storage
│ │ │ └── storage_test.go
│ │ ├── email/
│ │ │ ├── email.go # Email interface
│ │ │ ├── smtp.go # SMTP implementation
│ │ │ ├── template.go # Email templates
│ │ │ └── email_test.go
│ │ └── http/
│ │ ├── server.go # HTTP server setup
│ │ ├── middleware.go # HTTP middleware
│ │ ├── response.go # Response helpers
│ │ └── server_test.go
│ ├── core/ # Application foundation
│ │ ├── types/
│ │ │ ├── result.go # Result type (error handling)
│ │ │ ├── pagination.go # Pagination types
│ │ │ └── api.go # API request/response types
│ │ ├── errors/
│ │ │ ├── errors.go # Custom error types
│ │ │ └── codes.go # Error codes
│ │ ├── config/
│ │ │ ├── config.go # Configuration struct
│ │ │ ├── env.go # Environment loading
│ │ │ └── validation.go # Config validation
│ │ ├── middleware/
│ │ │ ├── auth.go # Authentication middleware
│ │ │ ├── cors.go # CORS middleware
│ │ │ ├── logging.go # Request logging middleware
│ │ │ ├── ratelimit.go # Rate limiting middleware
│ │ │ └── recovery.go # Panic recovery middleware
│ │ ├── security/
│ │ │ ├── crypto.go # Encryption utilities
│ │ │ ├── hash.go # Password hashing
│ │ │ ├── jwt.go # JWT token handling
│ │ │ └── random.go # Secure random generation
│ │ └── validation/
│ │ ├── validator.go # Input validation
│ │ └── rules.go # Validation rules
│ ├── shared/ # Shared utilities
│ │ ├── utils/
│ │ │ ├── time.go # Time utilities
│ │ │ ├── string.go # String utilities
│ │ │ ├── slice.go # Slice utilities
│ │ │ └── json.go # JSON utilities
│ │ ├── constants/
│ │ │ └── constants.go # Application constants
│ │ └── testutil/
│ │ ├── assert.go # Test assertions
│ │ ├── mock.go # Mock helpers
│ │ └── fixtures.go # Test fixtures
│ └── container/
│ ├── container.go # Dependency injection container
│ ├── provider.go # Service providers
│ └── wire.go # Manual dependency wiring
├── pkg/ # Public packages (if any)
│ └── client/ # Client library for this service
│ ├── client.go
│ └── types.go
├── api/ # API definitions
│ ├── openapi.yaml # OpenAPI specification
│ └── proto/ # Protocol buffer definitions (if needed)
├── scripts/ # Build and deployment scripts
│ ├── build.sh
│ ├── test.sh
│ ├── migrate.sh
│ └── docker-build.sh
├── deployments/ # Deployment configurations
│ ├── docker/
│ │ ├── Dockerfile
│ │ └── docker-compose.yml
│ └── k8s/ # Kubernetes manifests
├── docs/ # Documentation
│ ├── architecture.md
│ ├── deployment.md
│ └── api.md
├── .env.example
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
└── README.md
