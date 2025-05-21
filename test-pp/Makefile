
DOCKER_COMPOSE := docker compose -f docker-compose.yml

STOP := $(DOCKER_COMPOSE) down --remove-orphans; sleep 3; rm -rf data

.PHONY: run
run:
	$(DOCKER_COMPOSE) up -d xlayer-mock-l1-network
	$(DOCKER_COMPOSE) up -d xlayer-agg-db
	$(DOCKER_COMPOSE) up -d xlayer-bridge-db
	$(DOCKER_COMPOSE) up -d xlayer-agglayer-prover
	sleep 3
	$(DOCKER_COMPOSE) up -d xlayer-approve
	$(DOCKER_COMPOSE) up -d xlayer-seq

	sleep 3
	$(DOCKER_COMPOSE) up -d xlayer-agglayer
	sleep 10
	$(DOCKER_COMPOSE) up -d xlayer-cdk-node
	sleep 3
	$(DOCKER_COMPOSE) up -d xlayer-bridge-service
	$(DOCKER_COMPOSE) up -d xlayer-bridge-ui
	 
.PHONY: stop
stop: ## Stops all services
	$(STOP)

.PHONY: build-docker
build-docker: ## Build cdk-erigoin for sequencer and rpc
	docker build -t cdk-erigon -f ../Dockerfile.local ../
