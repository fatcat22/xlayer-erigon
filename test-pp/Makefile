
DOCKER_COMPOSE := docker compose -f docker-compose.yml

STOP := $(DOCKER_COMPOSE) down --remove-orphans; sleep 3; rm -rf data

.PHONY: run
run:
	$(DOCKER_COMPOSE) up -d xlayer-mock-l1-network
	sleep 3
	$(DOCKER_COMPOSE) up -d xlayer-approve
	$(DOCKER_COMPOSE) up -d xlayer-seq
	sleep 3
	$(DOCKER_COMPOSE) up -d xlayer-bridge-db
	$(DOCKER_COMPOSE) up -d xlayer-bridge-service
	$(DOCKER_COMPOSE) up -d xlayer-bridge-ui
	 
.PHONY: stop
stop: ## Stops all services
	$(STOP)
