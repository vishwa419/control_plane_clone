# pprof Goroutine Profiling Guide

This guide explains how to use pprof to monitor and debug goroutines in the control plane servers.

## Overview

All three servers (upload-server, consumer-server, grpc-server) have pprof integrated:
- **HTTP endpoints** for real-time profiling
- **SIGQUIT handler** for on-demand goroutine dumps

## Accessing pprof Endpoints

### Upload Server (Port 8080)
```bash
# Main pprof index
curl http://localhost:8080/debug/pprof/

# Goroutine dump (text format)
curl http://localhost:8080/debug/pprof/goroutine?debug=1

# Detailed goroutine dump
curl http://localhost:8080/debug/pprof/goroutine?debug=2

# Download goroutine profile (binary, use with go tool pprof)
curl http://localhost:8080/debug/pprof/goroutine -o goroutine.prof
```

### Consumer Server (Port 8081)
```bash
curl http://localhost:8081/debug/pprof/goroutine?debug=2
```

### gRPC Server (Port 8082 for HTTP)
```bash
curl http://localhost:8082/debug/pprof/goroutine?debug=2
```

## Using go tool pprof (Interactive)

```bash
# Start interactive pprof session
go tool pprof http://localhost:8080/debug/pprof/goroutine

# Or download and analyze
go tool pprof goroutine.prof

# In pprof interactive mode:
# - Type 'top' to see top goroutines by count
# - Type 'list <function>' to see source code
# - Type 'web' to generate a graph (requires graphviz)
# - Type 'help' for all commands
```

## SIGQUIT Goroutine Dumps

Send SIGQUIT to generate a file-based goroutine dump:

```bash
# Find the process ID
ps aux | grep upload-server

# Send SIGQUIT (Ctrl+\ in terminal)
kill -QUIT <pid>

# Or for all servers
killall -QUIT upload-server consumer-server grpc-server
```

The dump file will be created in the server's working directory:
- `goroutine-dump-upload-server-{timestamp}.txt`
- `goroutine-dump-consumer-server-{timestamp}.txt`
- `goroutine-dump-grpc-server-{timestamp}.txt`

## Expected Goroutines in Control Plane

### Upload Server
- **Main goroutine**: Server initialization and signal handling
- **HTTP server goroutine**: `server.ListenAndServe()` - handles HTTP requests
- **Request handler goroutines**: One per incoming HTTP request
- **gRPC broadcast client goroutines**: If GRPC_SERVER_URL is set
- **Signal handler goroutine**: `os/signal.loop()` for SIGQUIT/SIGINT

### Consumer Server
- **Main goroutine**: Server initialization
- **HTTP server goroutine**: `server.ListenAndServe()`
- **Request handler goroutines**: Per GET request for files
- **Signal handler goroutine**: Signal processing

### gRPC Server
- **Main goroutine**: Server initialization
- **HTTP server goroutine**: Broadcast endpoint server
- **gRPC server goroutine**: `grpcServer.Serve()` - handles gRPC connections
- **gRPC stream goroutines**: One per active consumer subscription
  - Each subscription has:
    - Main stream handler goroutine (blocked on `updateChan`)
    - Periodic ticker goroutine (updates last seen every 30s)
- **Signal handler goroutine**: Signal processing

## Analyzing Goroutine Dumps

### Common States

- `[running]`: Currently executing code
- `[sleep]`: Blocked on `time.Sleep()` or timer
- `[chan send]`: Blocked sending to a channel
- `[chan receive]`: Blocked receiving from a channel
- `[select]`: Blocked in a select statement
- `[syscall]`: Blocked in a system call
- `[IO wait]`: Waiting for I/O operations
- `[semacquire]`: Waiting on a mutex/semaphore

### Finding Issues

1. **Goroutine Leaks**: Count increases over time without decreasing
   ```bash
   # Compare dumps over time
   grep "Total goroutines" goroutine-dump-*.txt
   ```

2. **Deadlocks**: Many goroutines blocked on channels
   ```bash
   grep -c "\[chan" goroutine-dump-*.txt
   ```

3. **Blocked Operations**: Goroutines stuck in I/O
   ```bash
   grep "\[IO wait\]" goroutine-dump-*.txt
   ```

4. **Find Creation Points**: See where goroutines are spawned
   ```bash
   grep "created by" goroutine-dump-*.txt | sort | uniq -c
   ```

## Example: Monitoring Active gRPC Streams

When consumers are connected to the gRPC server, you'll see goroutines like:

```
goroutine 42 [chan receive]:
control-plane/internal/grpc.(*ControlPlaneService).SubscribeWorkerUpdates(...)
  /path/to/handlers.go:56
created by google.golang.org/grpc.(*Server).handleStream
  /go/pkg/mod/google.golang.org/grpc@v1.78.0/server.go:XXXX

goroutine 43 [sleep]:
time.Sleep(...)
control-plane/internal/grpc.(*ControlPlaneService).SubscribeWorkerUpdates.func1()
  /path/to/handlers.go:46
created by control-plane/internal/grpc.(*ControlPlaneService).SubscribeWorkerUpdates
  /path/to/handlers.go:41
```

Each consumer connection creates 2 goroutines:
1. Main stream handler (receiving from update channel)
2. Periodic ticker (updating last seen timestamp)

## Continuous Monitoring

For production monitoring, you can set up periodic dumps:

```bash
# Dump every 5 minutes
while true; do
  kill -QUIT $(pgrep -f upload-server)
  sleep 300
done
```

Or use pprof's web interface:
```bash
go tool pprof -http=:6060 http://localhost:8080/debug/pprof/goroutine
# Then visit http://localhost:6060 in your browser
```

## Troubleshooting

### pprof endpoints not accessible
- Check that `_ "net/http/pprof"` is imported
- Verify the router has `/debug/pprof/` path registered
- Check firewall/network settings

### SIGQUIT not creating dump file
- Verify signal handler is registered: `signal.Notify(quit, syscall.SIGQUIT)`
- Check file permissions in working directory
- Look for errors in server logs

### Too many goroutines
- Use `go tool pprof` to identify which functions create the most goroutines
- Check for goroutine leaks in request handlers
- Verify channels are properly closed
