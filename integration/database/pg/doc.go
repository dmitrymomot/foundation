// Package pg provides production-ready PostgreSQL connection management with migrations and health checking for SaaS applications.
//
// This package wraps the popular pgx PostgreSQL driver with application-level retry logic, connection pool
// optimization, and integrated database migration support using goose. It's designed for cloud-native applications
// that need reliable PostgreSQL connectivity with proper error handling and operational readiness.
//
// # Key Features
//
// The package provides comprehensive PostgreSQL integration:
//
//   - Connect: Creates a connection pool with retry logic and connection verification
//   - Migrate: Applies database schema migrations using goose with pgx integration
//   - Healthcheck: Returns a health check function for monitoring connectivity
//   - Error classification functions for common PostgreSQL error patterns
//
// Connection establishment uses exponential backoff retry logic to handle transient network issues
// and prevents thundering herd problems when multiple services restart simultaneously.
//
// # Configuration
//
// All configuration is handled through the Config struct with environment variable mapping:
//
//	type Config struct {
//		ConnectionString  string        `env:"PG_CONN_URL,required"`
//		MaxOpenConns      int32         `env:"PG_MAX_OPEN_CONNS" envDefault:"10"`
//		MaxIdleConns      int32         `env:"PG_MAX_IDLE_CONNS" envDefault:"5"`
//		HealthCheckPeriod time.Duration `env:"PG_HEALTHCHECK_PERIOD" envDefault:"1m"`
//		MaxConnIdleTime   time.Duration `env:"PG_MAX_CONN_IDLE_TIME" envDefault:"10m"`
//		MaxConnLifetime   time.Duration `env:"PG_MAX_CONN_LIFETIME" envDefault:"30m"`
//		RetryAttempts     int           `env:"PG_RETRY_ATTEMPTS" envDefault:"3"`
//		RetryInterval     time.Duration `env:"PG_RETRY_INTERVAL" envDefault:"5s"`
//		MigrationsPath    string        `env:"PG_MIGRATIONS_PATH" envDefault:"internal/db/migrations"`
//		MigrationsTable   string        `env:"PG_MIGRATIONS_TABLE" envDefault:"schema_migrations"`
//	}
//
// The default values are optimized for typical SaaS workloads, balancing performance, resource usage,
// and reliability for cloud-deployed applications.
//
// # Usage Example
//
//	package main
//
//	import (
//		"context"
//		"log"
//		"log/slog"
//		"os"
//		"time"
//
//		"github.com/dmitrymomot/foundation/integration/database/pg"
//	)
//
//	func main() {
//		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//		defer cancel()
//
//		// Load configuration from environment variables
//		cfg := pg.Config{
//			ConnectionString: "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
//			MaxOpenConns:     10,
//			MaxIdleConns:     5,
//			RetryAttempts:    3,
//			RetryInterval:    5 * time.Second,
//		}
//
//		// Create PostgreSQL connection pool with retry logic
//		pool, err := pg.Connect(ctx, cfg)
//		if err != nil {
//			log.Fatal("Failed to connect to PostgreSQL:", err)
//		}
//		defer pool.Close()
//
//		// Apply database migrations (optional)
//		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
//		if err := pg.Migrate(ctx, pool, cfg, logger); err != nil {
//			log.Fatal("Migration failed:", err)
//		}
//
//		// Use the connection pool
//		var name string
//		err = pool.QueryRow(ctx, "SELECT 'Hello, PostgreSQL!'").Scan(&name)
//		if err != nil {
//			log.Fatal("Query failed:", err)
//		}
//		log.Printf("Result: %s", name)
//	}
//
// # Database Migrations
//
// The package includes integrated migration support using goose with pgx compatibility:
//
//	// Apply migrations during application startup
//	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
//	if err := pg.Migrate(ctx, pool, cfg, logger); err != nil {
//		if errors.Is(err, pg.ErrMigrationsDirNotFound) {
//			log.Println("No migrations directory found, skipping migration")
//		} else {
//			log.Fatal("Migration failed:", err)
//		}
//	}
//
// The Migrate function handles the complex pgx->database/sql conversion required since goose
// doesn't natively support pgx, while preserving connection pool efficiency.
//
// # Health Checking
//
// The package provides a health check function suitable for Kubernetes readiness/liveness probes
// or HTTP health endpoints:
//
//	package main
//
//	import (
//		"net/http"
//
//		"github.com/dmitrymomot/foundation/integration/database/pg"
//	)
//
//	func main() {
//		// ... connection setup ...
//		pool, err := pg.Connect(ctx, cfg)
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		healthCheck := pg.Healthcheck(pool)
//
//		// Use in HTTP handler
//		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
//			if err := healthCheck(r.Context()); err != nil {
//				http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
//				return
//			}
//			w.WriteHeader(http.StatusOK)
//		})
//	}
//
// The health check performs a lightweight ping operation that verifies PostgreSQL connectivity
// without impacting database performance.
//
// # Error Handling
//
// The package defines domain-specific errors and provides helper functions for common PostgreSQL error patterns:
//
//	// Domain-specific errors
//	var (
//		ErrFailedToOpenDBConnection = errors.New("failed to open db connection")
//		ErrEmptyConnectionString    = errors.New("empty postgres connection string, use DATABASE_URL env var")
//		ErrHealthcheckFailed        = errors.New("healthcheck failed, connection is not available")
//		ErrFailedToParseDBConfig    = errors.New("failed to parse db config")
//		ErrFailedToApplyMigrations  = errors.New("failed to apply migrations")
//		ErrMigrationsDirNotFound    = errors.New("migrations directory not found")
//		ErrMigrationPathNotProvided = errors.New("migration path not provided")
//	)
//
//	// Error classification functions
//	isNotFound := pg.IsNotFoundError(err)        // Detects pgx.ErrNoRows
//	isDuplicate := pg.IsDuplicateKeyError(err)   // Detects unique constraint violations
//	isFKViolation := pg.IsForeignKeyViolationError(err) // Detects referential integrity violations
//	isTxClosed := pg.IsTxClosedError(err)        // Detects closed transaction usage
//
// These functions provide type-safe error checking for common database operation patterns,
// enabling proper retry logic and user-facing error messages.
//
// # Connection Pool Optimization
//
// The default connection pool settings are optimized for typical SaaS workloads:
//
//   - MaxOpenConns (10): Handles typical web traffic without overwhelming the database
//   - MaxIdleConns (5): Maintains warm connections for immediate availability
//   - HealthCheckPeriod (1m): Detects connection issues early without excessive overhead
//   - MaxConnIdleTime (10m): Prevents stale connections in load balancer environments
//   - MaxConnLifetime (30m): Handles database failovers and network changes
//
// For high-traffic applications, you may need to increase MaxOpenConns based on your
// concurrent request volume and database capacity. Monitor connection pool metrics
// to optimize these values for your specific workload.
//
// # Transaction Management
//
// The package works seamlessly with pgx transaction management, and provides
// small context helpers to propagate a transaction through your application
// layers so repositories can participate in the same DB transaction.
//
// Use WithTx to attach a pgx.Tx to a context and TxFromContext to retrieve it:
//
//	// In your business logic where you need atomic DB + enqueue (outbox-style):
//	func createOrder(ctx context.Context, pool *pgxpool.Pool, enq *queue.Enqueuer, params CreateOrderParams) error {
//		tx, err := pool.Begin(ctx)
//		if err != nil {
//			return err
//		}
//		defer tx.Rollback(ctx) // Safe even after commit
//
//		ctx = pg.WithTx(ctx, tx)
//
//		// 1) Domain writes using tx
//		var orderID uuid.UUID
//		err = tx.QueryRow(ctx, "INSERT INTO orders (customer_id, total) VALUES ($1,$2) RETURNING id", params.CustomerID, params.Total).Scan(&orderID)
//		if err != nil {
//			return err
//		}
//
//		// 2) Enqueue task within the same transaction
//		type OrderCreated struct { ID uuid.UUID `json:"id"` }
//		if err := enq.Enqueue(ctx, OrderCreated{ID: orderID}, queue.WithQueue("orders")); err != nil {
//			return err
//		}
//
//		return tx.Commit(ctx)
//	}
//
// In your repository/storage implementation, check the context for a transaction:
//
//	type Storage struct {
//		pool *pgxpool.Pool
//	}
//
//	func (s *Storage) CreateTask(ctx context.Context, task *queue.Task) error {
//		const q = `INSERT INTO tasks (id, queue, task_type, task_name, payload, status, priority, retry_count, max_retries, scheduled_at, created_at)
//			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
//		if tx, ok := pg.TxFromContext(ctx); ok {
//			_, err := tx.Exec(ctx, q,
//				task.ID, task.Queue, task.TaskType, task.TaskName,
//				task.Payload, task.Status, task.Priority, task.RetryCount,
//				task.MaxRetries, task.ScheduledAt, task.CreatedAt,
//			)
//			return err
//		}
//		_, err := s.pool.Exec(ctx, q,
//			task.ID, task.Queue, task.TaskType, task.TaskName,
//			task.Payload, task.Status, task.Priority, task.RetryCount,
//			task.MaxRetries, task.ScheduledAt, task.CreatedAt,
//		)
//		return err
//	}
//
// Use the provided error classification functions to handle transaction-specific errors
// and implement appropriate retry or recovery logic. Because workers run in separate
// sessions, they will not see uncommitted rows; once the transaction commits, the enqueued
// task becomes visible to workers.
package pg
