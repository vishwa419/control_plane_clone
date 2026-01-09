# gRPC Streaming Push Implementation

This document describes the gRPC streaming push mechanism implemented for the Cloudflare Workers clone control plane.

## Overview

The implementation adds a gRPC server that streams worker updates to consumers in real-time when files are uploaded. This replaces the polling mechanism with a push-based approach.

## Architecture

```
Upload Server → Broadcast Client → gRPC Server → Stream Manager → Consumers
```

1. **Upload Server**: Receives file uploads and triggers broadcasts
2. **Broadcast Client**: HTTP client that sends updates to gRPC server
3. **gRPC Server**: Manages consumer connections and broadcasts updates
4. **Stream Manager**: Tracks active consumer connections
5. **Consumers**: Subscribe to updates via gRPC streaming

## Components

### Protocol Buffers

- **Location**: `proto/control_plane.proto`
- **Generated Code**: `proto/gen/control_plane.pb.go`, `proto/gen/control_plane_grpc.pb.go`
- **Services**:
  - `SubscribeWorkerUpdates`: Server-side streaming RPC for worker updates
  - `RegisterConsumer`: Unary RPC for consumer registration

### gRPC Server

- **Main**: `cmd/grpc-server/main.go`
- **Server**: `internal/grpc/server.go`
- **Handlers**: `internal/grpc/handlers.go`
- **Stream Manager**: `internal/grpc/stream_manager.go`
- **Broadcast Client**: `internal/grpc/broadcast_client.go`

### Integration

- **Upload Handler**: `internal/handlers/upload.go` - Calls broadcast after successful upload
- **Config**: `internal/config/config.go` - Added GRPC configuration
- **Redis**: `internal/redis/schema.go` - Added consumer registration methods

## Key Features

1. **Real-time Updates**: Consumers receive updates immediately when files are uploaded
2. **Filtering**: Consumers can subscribe to specific workers or all workers
3. **Connection Management**: Stream manager tracks active connections
4. **Error Handling**: Graceful handling of disconnected consumers
5. **Cross-container Communication**: HTTP endpoint for upload server to trigger broadcasts

## Configuration

### Environment Variables

- `GRPC_PORT`: gRPC server port (default: 50051)
- `HTTP_PORT`: HTTP broadcast endpoint port (default: 8082)
- `GRPC_MAX_STREAMS`: Maximum number of concurrent streams (default: 1000)
- `GRPC_SERVER_URL`: URL of gRPC server HTTP endpoint (for upload server)

### Docker Compose

The gRPC server is added as a new service:
- Ports: 50051 (gRPC), 8082 (HTTP)
- Depends on: Redis
- Network: control-plane-network

## Usage

### Starting the Services

```bash
docker-compose up -d
```

### Running a Consumer Client

See `examples/consumer-client/main.go` for a complete example:

```bash
go run examples/consumer-client/main.go \
  -addr localhost:50051 \
  -consumer-id my-consumer \
  -workers worker1,worker2
```

### Uploading a File

```bash
curl -X POST http://localhost:8080/upload \
  -F "filename=my-worker" \
  -F "version=1.0.0" \
  -F "file=@worker.js"
```

The upload server will automatically broadcast the update to all connected consumers.

## Consumer Registration

Consumers are registered when they call `SubscribeWorkerUpdates`. The registration is tracked in:
- In-memory: Stream manager maintains active connections
- Redis: Consumer metadata stored for persistence (optional)

## Error Handling

- **Disconnected Consumers**: Automatically cleaned up when connection closes
- **Channel Buffer Full**: Updates are dropped if consumer is slow (logged as warning)
- **Reconnection**: Consumer clients should implement reconnection logic

## Future Enhancements

1. **Redis Pub/Sub**: Use Redis for cross-instance broadcasting
2. **Consumer Health Checks**: Periodic health checks for registered consumers
3. **Metrics**: Add Prometheus metrics for monitoring
4. **Authentication**: Add TLS and authentication for gRPC connections
5. **Rate Limiting**: Add rate limiting for broadcasts

## Testing

To test the implementation:

1. Start all services: `docker-compose up -d`
2. Run consumer client: `go run examples/consumer-client/main.go`
3. Upload a file: `curl -X POST http://localhost:8080/upload ...`
4. Verify consumer receives the update

## Notes

- Proto files are manually created. Regenerate with `protoc` for production use.
- The broadcast client uses HTTP for cross-container communication.
- Stream manager uses buffered channels (100 updates) to handle bursts.
