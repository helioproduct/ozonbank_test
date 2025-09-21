ENV_FILE=.env
COMPOSE_FILE=deployments/docker-compose.yaml
DC=docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE)

.PHONY: up down clear

up:
	$(DC) up -d

down:
	$(DC) down

clear:
	$(DC) down -v
