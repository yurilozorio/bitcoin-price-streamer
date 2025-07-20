# Bitcoin Price Streamer Makefile

# Variables
DOCKER_IMAGE=bitcoin-price-streamer

.PHONY: build run test-unit test-integration test-all docker-build docker-run docker-clean

build:
	go build -o $(DOCKER_IMAGE) main.go

run: 
	go run main.go

test-unit:
	go test -v ./internal/...

test-integration:
	go test -v ./tests/integration_test.go

test-all: test-unit test-integration

# Docker commands
docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	docker run -p 8080:8080 $(DOCKER_IMAGE)

docker-stop:
	docker stop $(shell docker ps -q --filter ancestor=$(DOCKER_IMAGE)) 2>/dev/null || true

docker-clean: docker-stop
	docker rmi $(DOCKER_IMAGE) 2>/dev/null || true
