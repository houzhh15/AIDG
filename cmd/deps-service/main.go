package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "/app/config/commands.yaml", "Path to config file")
	port := flag.Int("port", 8080, "HTTP server port")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	// Show version if requested
	if *showVersion {
		fmt.Printf("Command Executor Service v%s\n", Version)
		os.Exit(0)
	}

	// Load configuration
	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize components
	validator := NewValidator(config)
	executor := NewExecutor(config)
	auditLogger := NewAuditLogger(config.Security.AuditLogPath)
	limiter := NewConcurrencyLimiter(config)

	// Create HTTP handler
	handler := NewHandler(validator, executor, auditLogger, limiter)

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: handler,
	}

	// Setup graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGTERM, syscall.SIGINT)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting Command Executor Service on port %d\n", *port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for stop signal
	<-stopChan

	// Graceful shutdown
	log.Println("Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
