// Package config provides type-safe environment variable loading with caching
// using Go generics. Each configuration type is loaded once and cached for
// subsequent calls.
//
// The package automatically loads .env files on first use and uses the
// caarlos0/env library for parsing environment variables into struct fields.
//
// Basic usage:
//
//	import "github.com/dmitrymomot/foundation/core/config"
//
//	type DatabaseConfig struct {
//		Host     string `env:"DB_HOST" envDefault:"localhost"`
//		Port     int    `env:"DB_PORT" envDefault:"5432"`
//		Username string `env:"DB_USER,required"`
//		Password string `env:"DB_PASS,required"`
//	}
//
//	func main() {
//		var db DatabaseConfig
//
//		// Load with error handling
//		if err := config.Load(&db); err != nil {
//			log.Fatal(err)
//		}
//
//		// Or panic on failure (useful for startup)
//		config.MustLoad(&db)
//	}
//
// # Caching Behavior
//
// Each configuration type is loaded only once per application lifetime:
//
//	var cfg1 DatabaseConfig
//	config.Load(&cfg1) // Loads from environment
//
//	var cfg2 DatabaseConfig
//	config.Load(&cfg2) // Returns cached value, cfg1 == cfg2
//
// Different types are cached independently:
//
//	type ServerConfig struct {
//		Port int `env:"PORT" envDefault:"8080"`
//	}
//
//	type RedisConfig struct {
//		URL string `env:"REDIS_URL,required"`
//	}
//
//	// Each type has its own cache entry
//	config.MustLoad(&ServerConfig{})
//	config.MustLoad(&RedisConfig{})
package config
