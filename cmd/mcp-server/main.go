package main

import (
	"log"
	"net/http"

	"mcp-sse-go/internal/server"
)

func main() {
	srv := server.New()
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", srv); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
