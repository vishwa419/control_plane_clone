package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"control-plane/proto/gen"
)

func main() {
	// Parse command line flags
	grpcAddr := flag.String("addr", "localhost:50051", "gRPC server address")
	consumerID := flag.String("consumer-id", "consumer-1", "Consumer ID")
	workerNames := flag.String("workers", "", "Comma-separated list of worker names to subscribe to (empty = all)")
	flag.Parse()

	// Connect to gRPC server
	conn, err := grpc.NewClient(*grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	client := gen.NewControlPlaneClient(conn)

	// Create subscribe request
	req := &gen.SubscribeRequest{
		ConsumerId: *consumerID,
	}

	// Parse worker names if provided
	if *workerNames != "" {
		// Split comma-separated worker names
		// For simplicity, we'll just set it as a single worker
		// In production, parse the comma-separated string
		req.WorkerNames = []string{*workerNames}
	}

	log.Printf("Connecting to gRPC server at %s as consumer %s", *grpcAddr, *consumerID)

	// Subscribe to worker updates
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.SubscribeWorkerUpdates(ctx, req)
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Println("Subscribed to worker updates. Waiting for updates...")

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Receive updates in a goroutine
	updateChan := make(chan *gen.WorkerUpdate, 10)
	errChan := make(chan error, 1)

	go func() {
		for {
			update, err := stream.Recv()
			if err != nil {
				errChan <- err
				return
			}
			updateChan <- update
		}
	}()

	// Process updates
	for {
		select {
		case update := <-updateChan:
			handleUpdate(update)
		case err := <-errChan:
			log.Printf("Stream error: %v", err)
			log.Println("Attempting to reconnect in 5 seconds...")
			time.Sleep(5 * time.Second)
			// Reconnect logic would go here
			return
		case <-quit:
			log.Println("Shutting down...")
			cancel()
			return
		}
	}
}

func handleUpdate(update *gen.WorkerUpdate) {
	log.Printf("Received update:")
	log.Printf("  Worker: %s", update.WorkerName)
	log.Printf("  Version: %s", update.Version)
	log.Printf("  Checksum: %s", update.Checksum)
	log.Printf("  Size: %d bytes", update.Size)
	log.Printf("  File Path: %s", update.FilePath)
	log.Printf("  Timestamp: %d", update.Timestamp)
	log.Printf("  Code Length: %d bytes", len(update.WorkerCode))
	log.Println("---")

	// Here you would:
	// 1. Validate the checksum
	// 2. Store the worker code to disk or memory
	// 3. Deploy the worker to your runtime
	// 4. Update your internal state

	// Example: Save to file
	saveWorkerCode(update)
}

func saveWorkerCode(update *gen.WorkerUpdate) {
	// Create directory structure: workers/{worker_name}/{version}
	dir := "workers/" + update.WorkerName + "/" + update.Version
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create directory %s: %v", dir, err)
		return
	}

	// Save worker code
	filePath := dir + "/worker.js"
	if err := os.WriteFile(filePath, update.WorkerCode, 0644); err != nil {
		log.Printf("Failed to save worker code: %v", err)
		return
	}

	log.Printf("Saved worker code to %s", filePath)
}
