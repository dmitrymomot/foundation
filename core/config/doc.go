// Package config provides utilities for loading and caching application configuration
// from environment variables with support for .env files and type-safe generics.
// It ensures that configuration structs are loaded only once per type, providing
// both performance benefits and consistent configuration state across the application.
//
// # Features
//
//   - Type-safe configuration loading using Go generics
//   - Automatic .env file loading for development environments
//   - Singleton pattern ensures each config type is loaded only once
//   - Thread-safe caching with concurrent access support
//   - Comprehensive error handling with descriptive error types
//   - Support for required and optional fields with default values
//   - Built on top of popular env parsing libraries for reliability
//
// # Usage
//
// The package provides two main functions for loading configuration:
//
//	import "github.com/dmitrymomot/gokit/core/config"
//
//	// Load configuration with error handling
//	err := config.Load(&myConfig)
//	if err != nil {
//		// Handle error
//	}
//
//	// Load configuration with panic on failure
//	config.MustLoad(&myConfig)
//
// # Basic Configuration
//
// Define configuration structs using struct tags to map environment variables:
//
//	type DatabaseConfig struct {
//		Host     string `env:"DB_HOST" envDefault:"localhost"`
//		Port     int    `env:"DB_PORT" envDefault:"5432"`
//		Username string `env:"DB_USER,required"`
//		Password string `env:"DB_PASS,required"`
//		DBName   string `env:"DB_NAME,required"`
//		SSLMode  string `env:"DB_SSLMODE" envDefault:"disable"`
//	}
//
//	func main() {
//		var db DatabaseConfig
//		if err := config.Load(&db); err != nil {
//			log.Fatal("Failed to load database config:", err)
//		}
//
//		// Use configuration
//		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
//			db.Host, db.Port, db.Username, db.Password, db.DBName, db.SSLMode)
//	}
//
// # Application Configuration
//
// Structure your application configuration using nested structs for organization:
//
//	type ServerConfig struct {
//		Port         int           `env:"PORT" envDefault:"8080"`
//		Host         string        `env:"HOST" envDefault:"localhost"`
//		ReadTimeout  time.Duration `env:"READ_TIMEOUT" envDefault:"30s"`
//		WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"30s"`
//	}
//
//	type RedisConfig struct {
//		URL      string `env:"REDIS_URL,required"`
//		Password string `env:"REDIS_PASSWORD"`
//		DB       int    `env:"REDIS_DB" envDefault:"0"`
//	}
//
//	type AppConfig struct {
//		Environment string `env:"APP_ENV" envDefault:"development"`
//		Debug       bool   `env:"DEBUG" envDefault:"false"`
//		LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
//		Secret      string `env:"APP_SECRET,required"`
//	}
//
//	func loadConfiguration() {
//		var server ServerConfig
//		var redis RedisConfig
//		var app AppConfig
//
//		// Load each configuration section
//		config.MustLoad(&server)
//		config.MustLoad(&redis)
//		config.MustLoad(&app)
//
//		// Configurations are now available globally through the cache
//	}
//
// # Environment File Support
//
// The package automatically loads .env files during the first configuration load:
//
//	# .env file example
//	DB_HOST=localhost
//	DB_PORT=5432
//	DB_USER=myapp
//	DB_PASS=secretpassword
//	DB_NAME=myapp_db
//	REDIS_URL=redis://localhost:6379
//	APP_SECRET=your-secret-key-here
//	LOG_LEVEL=debug
//
//	// Configuration loading will automatically use these values
//	var config AppConfig
//	config.Load(&config)
//
// # Configuration Caching
//
// Each configuration type is loaded only once and cached for subsequent calls:
//
//	// First call loads from environment
//	var config1 DatabaseConfig
//	config.Load(&config1) // Loads from environment variables
//
//	// Subsequent calls return cached version
//	var config2 DatabaseConfig
//	config.Load(&config2) // Returns cached version, no environment parsing
//
//	// config1 and config2 contain identical values
//
// # Multiple Configuration Types
//
// Different configuration types are managed independently:
//
//	type DatabaseConfig struct {
//		Host string `env:"DB_HOST" envDefault:"localhost"`
//		Port int    `env:"DB_PORT" envDefault:"5432"`
//	}
//
//	type CacheConfig struct {
//		URL string `env:"REDIS_URL,required"`
//		TTL int    `env:"CACHE_TTL" envDefault:"3600"`
//	}
//
//	func initializeServices() {
//		var db DatabaseConfig
//		var cache CacheConfig
//
//		// Each type is loaded and cached separately
//		config.MustLoad(&db)
//		config.MustLoad(&cache)
//	}
//
// # Error Handling
//
// The package provides specific error types for different failure scenarios:
//
//	import "errors"
//
//	var config MyConfig
//	err := config.Load(&config)
//	if err != nil {
//		switch {
//		case errors.Is(err, config.ErrParsingConfig):
//			// Handle environment variable parsing errors
//			log.Error("Invalid environment variables:", err)
//		case errors.Is(err, config.ErrNilPointer):
//			// Handle nil pointer errors
//			log.Error("Configuration pointer is nil:", err)
//		default:
//			// Handle other errors
//			log.Error("Configuration loading failed:", err)
//		}
//	}
//
// # Advanced Field Tags
//
// Utilize various struct tags for flexible configuration options:
//
//	type AdvancedConfig struct {
//		// Required field
//		APIKey string `env:"API_KEY,required"`
//
//		// Optional with default
//		Timeout time.Duration `env:"TIMEOUT" envDefault:"30s"`
//
//		// Multiple possible environment variable names
//		Port int `env:"PORT,SERVER_PORT" envDefault:"8080"`
//
//		// Boolean from string
//		EnableFeature bool `env:"FEATURE_ENABLED" envDefault:"false"`
//
//		// Slice from comma-separated values
//		AllowedOrigins []string `env:"ALLOWED_ORIGINS" envSeparator:","`
//
//		// Map from key=value pairs
//		Headers map[string]string `env:"HEADERS" envSeparator:","`
//	}
//
// # Configuration Validation
//
// Combine with validation libraries for comprehensive configuration checking:
//
//	import "github.com/go-playground/validator/v10"
//
//	type ValidatedConfig struct {
//		Port     int    `env:"PORT" envDefault:"8080" validate:"min=1,max=65535"`
//		Host     string `env:"HOST" envDefault:"localhost" validate:"required"`
//		LogLevel string `env:"LOG_LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
//	}
//
//	func loadValidatedConfig() (*ValidatedConfig, error) {
//		var cfg ValidatedConfig
//		if err := config.Load(&cfg); err != nil {
//			return nil, err
//		}
//
//		validate := validator.New()
//		if err := validate.Struct(&cfg); err != nil {
//			return nil, fmt.Errorf("configuration validation failed: %w", err)
//		}
//
//		return &cfg, nil
//	}
//
// # Microservices Configuration
//
// Structure configuration for microservices with shared and service-specific settings:
//
//	// Shared configuration
//	type BaseConfig struct {
//		Environment string `env:"ENVIRONMENT" envDefault:"development"`
//		LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
//		Metrics     bool   `env:"ENABLE_METRICS" envDefault:"true"`
//	}
//
//	// Service-specific configuration
//	type UserServiceConfig struct {
//		BaseConfig
//		DatabaseURL string `env:"USER_DB_URL,required"`
//		CacheURL    string `env:"USER_CACHE_URL,required"`
//		JWTSecret   string `env:"JWT_SECRET,required"`
//	}
//
//	type OrderServiceConfig struct {
//		BaseConfig
//		DatabaseURL  string `env:"ORDER_DB_URL,required"`
//		PaymentURL   string `env:"PAYMENT_SERVICE_URL,required"`
//		EmailService string `env:"EMAIL_SERVICE_URL,required"`
//	}
//
// # Thread Safety
//
// All operations are thread-safe and can be called concurrently:
//
//	func initializeConcurrently() {
//		var wg sync.WaitGroup
//
//		wg.Add(2)
//		go func() {
//			defer wg.Done()
//			var db DatabaseConfig
//			config.MustLoad(&db)
//		}()
//
//		go func() {
//			defer wg.Done()
//			var cache CacheConfig
//			config.MustLoad(&cache)
//		}()
//
//		wg.Wait()
//		// Both configurations loaded safely
//	}
//
// # Best Practices
//
//   - Use MustLoad for critical configurations that are required at startup
//   - Use Load with error handling for optional or runtime configurations
//   - Group related configuration fields into separate structs for better organization
//   - Use meaningful default values to make development easier
//   - Validate configuration values after loading to catch invalid settings early
//   - Keep sensitive values like secrets and passwords in environment variables, not in .env files
//   - Use descriptive environment variable names with consistent prefixes
//   - Document required environment variables in your application's README
//
// # Environment Variable Naming
//
// Follow consistent naming conventions for environment variables:
//
//	// Good: Consistent prefixes and clear names
//	DB_HOST=localhost
//	DB_PORT=5432
//	DB_USER=myapp
//	REDIS_URL=redis://localhost:6379
//	SMTP_HOST=smtp.gmail.com
//	JWT_SECRET=your-secret-here
//
//	// Avoid: Inconsistent or unclear names
//	HOST=localhost        // Too generic
//	database_port=5432    // Inconsistent case
//	r_url=redis://...     // Unclear abbreviation
package config
