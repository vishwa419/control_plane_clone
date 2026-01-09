package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"control-plane/internal/config"
	"control-plane/internal/handlers"
	"control-plane/internal/redis"
	"control-plane/internal/storage"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize storage
	fileStorage, err := storage.NewLocalStorage("/tmp/test-files")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize Redis client
	redisClient, err := redis.NewClient("localhost", "6379", "")
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize handlers (without grpc to avoid protobuf issues)
	uploadHandler := handlers.NewUploadHandler(fileStorage, redisClient, cfg)

	// Setup router
	mux := http.NewServeMux()
	mux.HandleFunc("/upload", uploadHandler.HandleUpload)
	mux.HandleFunc("/health", uploadHandler.HandleHealth)
	
	// Setup pprof endpoints
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

	// Start server
	server := &http.Server{
		Addr:    ":18080",
		Handler: mux,
	}

	go func() {
		log.Printf("Test pprof server starting on port 18080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	sig := <-quit
	if sig == syscall.SIGQUIT {
		dumpGoroutines("test-pprof-server")
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
	}

	log.Println("Shutting down server...")
	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func dumpGoroutines(serverName string) {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("goroutine-dump-%s-%s.txt", serverName, timestamp)

	file, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create goroutine dump file: %v", err)
		fmt.Fprintf(os.Stderr, "\n=== Goroutine Dump for %s at %s ===\n", serverName, time.Now().Format(time.RFC3339))
		pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
		return
	}
	defer file.Close()

	fmt.Fprintf(file, "=== Goroutine Dump for %s at %s ===\n", serverName, time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "Total goroutines: %d\n\n", runtime.NumGoroutine())
	pprof.Lookup("goroutine").WriteTo(file, 2)

	log.Printf("Goroutine dump written to %s", filename)
}
