#!/bin/bash

echo "Starting MCP Server with telemetry..."
PORT=8083 ./bin/mcp-server &
SERVER_PID=$!

# Wait for server to start
sleep 3

echo "Testing health endpoint..."
curl -s http://localhost:8083/health
echo ""

echo "Testing metrics endpoint..."
curl -s http://localhost:8083/metrics | grep -E "(http_requests|mcp_sessions|go_goroutines)" | head -5
echo ""

echo "Making a few requests to generate metrics..."
curl -s http://localhost:8083/health > /dev/null
curl -s http://localhost:8083/health > /dev/null
curl -s http://localhost:8083/health > /dev/null

echo "Checking updated metrics..."
curl -s http://localhost:8083/metrics | grep "http_requests_total" | head -3
echo ""

echo "Stopping server..."
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null

echo "Telemetry test completed!"