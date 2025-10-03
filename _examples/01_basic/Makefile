.PHONY: help env up down logs clean rebuild run dev check-docker sqlc

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

check-docker:
	@command -v docker >/dev/null 2>&1 || { echo "Error: Docker is not installed"; exit 1; }
	@docker info >/dev/null 2>&1 || { echo "Error: Docker is not running"; exit 1; }

env: ## Create .env file from .env.example
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "✓ .env file created from .env.example"; \
	else \
		echo "✓ .env file already exists"; \
	fi

up: check-docker env ## Start PostgreSQL database
	@echo "Starting PostgreSQL..."
	@docker-compose up -d
	@echo ""
	@echo "✓ PostgreSQL started successfully!"
	@echo ""
	@echo "Database connection:"
	@echo "  - PostgreSQL: localhost:5432"
	@echo ""
	@echo "To run the app: make run"
	@echo "To view logs: make logs"

down: check-docker ## Stop PostgreSQL database
	@docker-compose down

rebuild: check-docker env ## Rebuild and restart PostgreSQL from scratch
	@echo "Rebuilding PostgreSQL..."
	@docker-compose down
	@docker-compose up -d --force-recreate
	@echo ""
	@echo "✓ PostgreSQL rebuilt and started successfully!"
	@echo ""
	@echo "Database connection:"
	@echo "  - PostgreSQL: localhost:5432"

logs: check-docker ## View PostgreSQL logs
	@docker-compose logs -f

clean: check-docker ## Stop database and remove containers, volumes, and data
	@docker-compose down -v --remove-orphans
	@echo "✓ Database stopped and all resources removed"

run: env ## Run the application locally
	@echo "Starting application..."
	@go run .

dev: check-docker env ## Start database and run application
	@echo "Starting PostgreSQL..."
	@docker-compose up -d
	@echo ""
	@echo "✓ PostgreSQL started successfully!"
	@echo ""
	@echo "Waiting for database to be ready..."
	@sleep 3
	@echo ""
	@echo "Starting application..."
	@go run .

sqlc: ## Generate sqlc code from queries
	@echo "Generating sqlc code..."
	@sqlc generate -f db/sqlc.yaml
	@echo "✓ sqlc code generated successfully!"
