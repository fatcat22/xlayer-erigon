
DOCKER_COMPOSE := docker compose -f docker-compose.yml

STOP := $(DOCKER_COMPOSE) down --remove-orphans; sleep 3; rm -rf data

INIT_BRIDGE_ACCOUNT := cast send -f 0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534  --private-key 0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2 --value 0.01ether 0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266 --legacy --rpc-url http://127.0.0.1:8123

.PHONY: run
run: build-docker
	#$(DOCKER_COMPOSE) up -d xlayer-mock-l1-network
	$(DOCKER_COMPOSE) up -d xlayer-executor
	$(DOCKER_COMPOSE) up -d xlayer-executor-2
	$(DOCKER_COMPOSE) up -d xlayer-executor-3
	$(DOCKER_COMPOSE) up -d xlayer-executor-4
	$(DOCKER_COMPOSE) up -d xlayer-executor-5
	$(DOCKER_COMPOSE) up -d xlayer-executor-6
	$(DOCKER_COMPOSE) up -d xlayer-agg-db
	$(DOCKER_COMPOSE) up -d xlayer-bridge-db
	$(DOCKER_COMPOSE) up -d xlayer-pool-db
	$(DOCKER_COMPOSE) up -d xlayer-da-db
	$(DOCKER_COMPOSE) up -d xlayer-prover-true

	sleep 5
	$(DOCKER_COMPOSE) up -d xlayer-approve
	$(DOCKER_COMPOSE) up -d xlayer-seq
	$(DOCKER_COMPOSE) up -d xlayer-pool-manager
	sleep 5
	$(DOCKER_COMPOSE) up -d xlayer-ds

	sleep 5
	$(DOCKER_COMPOSE) up -d xlayer-rpc
	sleep 5
	$(DOCKER_COMPOSE) up -d xlayer-seqs
	$(DOCKER_COMPOSE) up -d xlayer-agg
	$(DOCKER_COMPOSE) up -d xlayer-da

	sleep 30
	$(DOCKER_COMPOSE) up -d xlayer-bridge-service
	$(DOCKER_COMPOSE) up -d xlayer-bridge-ui
	$(INIT_BRIDGE_ACCOUNT)

run-mock:
	$(DOCKER_COMPOSE) stop xlayer-prover-true
	sleep 3
	$(DOCKER_COMPOSE) up xlayer-seqs -d
	sleep 3
	$(DOCKER_COMPOSE) up xlayer-agg -d
	sleep 10
	$(DOCKER_COMPOSE) up -d xlayer-prover-mock

run-true:
	$(DOCKER_COMPOSE) stop xlayer-prover-mock
	sleep 3
	$(DOCKER_COMPOSE) up xlayer-seqs -d
	sleep 3
	$(DOCKER_COMPOSE) up xlayer-agg -d
	sleep 10
	$(DOCKER_COMPOSE) up -d xlayer-prover-true
	
.PHONY: stop
stop: ## Stops all services
	$(STOP)

.PHONY: build-docker
build-docker: ## Build cdk-erigoin for sequencer and rpc
	docker build -t cdk-erigon -f ../Dockerfile.local ../