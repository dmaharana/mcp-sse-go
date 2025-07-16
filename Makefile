
.PHONY: build clean test run docker-build docker-up docker-down monitoring-up monitoring-down

# Build the application
build:
	go build -o ./bin/mcp-server -ldflags="-s -w" ./cmd/mcp-server

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test -v ./...

# Run the application
run: build
	./bin/mcp-server

# Docker commands
docker-build:
	docker build -t mcp-server .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# Monitoring commands
monitoring-up:
	docker-compose up -d prometheus grafana

monitoring-down:
	docker-compose stop prometheus grafana
