
.PHONY: build run

build:
	go build -o ./bin/mcp-server -ldflags="-s -w" ./cmd/mcp-server

run: build
	./bin/mcp-server
