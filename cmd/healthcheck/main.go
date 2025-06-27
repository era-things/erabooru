package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	// Just makes HTTP request - no config loading, no DB connections
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get("http://localhost:8080/health")
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}

	os.Exit(0)
}
