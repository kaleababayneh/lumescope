# Makefile for LumeScope

.PHONY: build docker-build docker-run docker-run-local docker-run-mainnet docker-run-testnet docker-rebuild docker-stop docker-rm

# Build the Go binary locally
build:
	go build -o bin/lumescope ./cmd/lumescope

# Build the Docker image
docker-build:
	docker build -t lumescope .

# Run the Docker container (ephemeral, for local development)
docker-run:
	docker run -d -p 18080:18080 --name lumescope lumescope

# Alias for docker-run (ephemeral local development)
docker-run-local: docker-run

# Run the Docker container for Mainnet (with persistent volume)
docker-run-mainnet:
	docker run -d -p 18080:18080 --name lumescope \
		-e LUMERA_API_BASE=https://lcd.lumera.io \
		-v lumescope_data_mainnet:/var/lib/postgresql/data \
		lumescope

# Run the Docker container for Testnet (with persistent volume)
docker-run-testnet:
	docker run -d -p 18080:18080 --name lumescope \
		-e LUMERA_API_BASE=https://lcd.testnet.lumera.io \
		-v lumescope_data_testnet:/var/lib/postgresql/data \
		lumescope

# Stop the Docker container (ignore error if not running)
docker-stop:
	-docker stop lumescope

# Remove the Docker container (ignore error if missing)
docker-rm:
	-docker rm lumescope

# Rebuild: stop, remove, build image, and run new container (ephemeral)
docker-rebuild: docker-stop docker-rm docker-build