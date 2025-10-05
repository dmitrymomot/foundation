.PHONY: up down

up: ## Start docker and run application
	@command -v docker >/dev/null 2>&1 || { echo "Error: Docker is not installed"; exit 1; }
	@docker info >/dev/null 2>&1 || { echo "Error: Docker is not running"; exit 1; }
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "✓ .env file created from .env.example"; \
	fi
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

down: ## Stop docker and remove all resources
	@docker-compose down -v --remove-orphans --rmi=local
	@echo "✓ Docker stopped and all resources removed"
