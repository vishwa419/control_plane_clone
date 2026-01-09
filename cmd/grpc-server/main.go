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
	grpcServer "control-plane/internal/grpc"
	"control-plane/internal/redis"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize Redis client (for future consumer registry)
	redisClient, err := redis.NewClient(cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize gRPC server
	grpcPort := cfg.GRPC.Port
	server := grpcServer.NewServer(grpcPort)

	// Setup HTTP server for broadcast endpoint
	httpMux := http.NewServeMux()
	server.SetupHTTPBroadcastEndpoint(httpMux)
	
	// Setup pprof endpoints
	httpMux.Handle("/debug/pprof/", http.DefaultServeMux)
	
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8082" // Default HTTP port for broadcast endpoint
	}

	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: httpMux,
	}

	// Start HTTP server for broadcast endpoint
	go func() {
		log.Printf("HTTP broadcast endpoint starting on port %s", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start gRPC server
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	log.Printf("gRPC server running on port %s", grpcPort)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	
	sig := <-quit
	if sig == syscall.SIGQUIT {
		dumpGoroutines("grpc-server")
		// After dump, wait for shutdown signal
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
	}

	log.Println("Shutting down servers...")
	server.Stop()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Servers exited")
}

// dumpGoroutines writes a goroutine dump to stderr
func dumpGoroutines(serverName string) {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("goroutine-dump-%s-%s.txt", serverName, timestamp)
	
	file, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create goroutine dump file: %v", err)
		// Fallback to stderr
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
