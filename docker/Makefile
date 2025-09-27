# YSF Nexus Docker Makefile

.PHONY: help simple full build logs restart stop clean status health

help: ## Show this help message
	@echo "YSF Nexus Docker Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

simple: ## Start simple setup (reflector + web only)
	@echo "Starting simple YSF Nexus setup..."
	@cp config.yaml config.yaml.local 2>/dev/null || true
	@echo "Edit config.yaml.local if needed, then run: docker-compose up -d"

full: ## Start full monitoring setup (with MQTT, Prometheus, Grafana)
	@echo "Starting full YSF Nexus monitoring stack..."
	@cp config.full.yaml config.yaml.local 2>/dev/null || true
	@echo "Edit config.yaml.local if needed, then run: docker-compose -f docker-compose.full.yml up -d"

build: ## Build the Docker image
	docker-compose build --no-cache

up: ## Start services (detached)
	docker-compose up -d

up-full: ## Start full monitoring services (detached)
	docker-compose -f docker-compose.full.yml up -d

logs: ## Show logs from YSF Nexus
	docker-compose logs -f ysf-nexus

logs-all: ## Show logs from all services
	docker-compose logs -f

restart: ## Restart YSF Nexus service
	docker-compose restart ysf-nexus

restart-all: ## Restart all services
	docker-compose restart

stop: ## Stop all services
	docker-compose down

stop-full: ## Stop full monitoring services
	docker-compose -f docker-compose.full.yml down

clean: ## Remove containers and networks (keeps volumes)
	docker-compose down --remove-orphans

clean-all: ## Remove everything including volumes (DESTRUCTIVE)
	docker-compose down -v --remove-orphans
	docker-compose -f docker-compose.full.yml down -v --remove-orphans

status: ## Show container status
	docker-compose ps

health: ## Check container health
	docker-compose exec ysf-nexus wget -qO- http://localhost:8080/api/health || echo "Health check failed"

shell: ## Open shell in YSF Nexus container
	docker-compose exec ysf-nexus sh

# Development helpers
dev-build: ## Build and start for development
	docker-compose build && docker-compose up

dev-rebuild: ## Force rebuild and restart
	docker-compose down && docker-compose build --no-cache && docker-compose up -d

# Monitoring helpers
grafana: ## Open Grafana in browser (macOS)
	open http://localhost:3000

prometheus: ## Open Prometheus in browser (macOS)
	open http://localhost:9091

dashboard: ## Open YSF Nexus dashboard in browser (macOS)
	open http://localhost:8080