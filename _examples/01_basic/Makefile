.PHONY: help env up down logs clean rebuild check-docker

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

up: check-docker env ## Create .env and start all services
	@echo "Starting services..."
	@docker-compose up -d
	@echo ""
	@echo "✓ Services started successfully!"
	@echo ""
	@echo "Available services:"
	@echo "  - PostgreSQL: localhost:5432"
	@echo "  - App: http://localhost:8080"
	@echo ""
	@echo "To view logs: make logs"

down: check-docker ## Stop all services
	@docker-compose down

rebuild: check-docker env ## Rebuild and restart all services from scratch
	@echo "Rebuilding services..."
	@docker-compose down
	@docker-compose build --no-cache
	@docker-compose up -d
	@echo ""
	@echo "✓ Services rebuilt and started successfully!"
	@echo ""
	@echo "Available services:"
	@echo "  - PostgreSQL: localhost:5432"
	@echo "  - App: http://localhost:8080"
	@echo ""
	@echo "To view logs: make logs"

logs: check-docker ## View logs
	@docker-compose logs -f

clean: check-docker ## Stop services and remove containers, volumes, networks, and images
	@docker-compose down -v --rmi local --remove-orphans
	@echo "✓ Services stopped and all resources removed"
