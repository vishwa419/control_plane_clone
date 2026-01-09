# Understanding pprof and Why Some Goroutines Don't Appear

## What is pprof?

**pprof** (performance profiler) is Go's built-in profiling tool that captures a **snapshot** of your program's state at a specific moment in time. It shows:

1. **Active goroutines** - Currently running or blocked
2. **Stack traces** - Where each goroutine is in the code
3. **Goroutine states** - What they're waiting on (channels, I/O, locks, etc.)

### Key Point: pprof is a SNAPSHOT
- It only shows goroutines that exist **at the moment you capture the profile**
- Goroutines that complete quickly won't appear if you capture after they finish
- It's like taking a photo - you only see what's in the frame at that instant

## Why Checksum Calculation Doesn't Appear

Looking at `internal/handlers/upload.go` lines 185-214:

```go
func (h *UploadHandler) processFileUpload(...) {
    var wg sync.WaitGroup
    
    // Calculate checksum in parallel
    wg.Add(1)
    go func() {
        defer wg.Done()
        hash := sha256.Sum256(fileData)  // Fast CPU operation
        checksum = hex.EncodeToString(hash[:])
    }()
    
    // Write file in parallel
    wg.Add(1)
    go func() {
        defer wg.Done()
        filePath, writeErr = h.storage.Save(...)
    }()
    
    wg.Wait()  // Wait for both to complete
}
```

**Why it doesn't show up:**
1. The checksum goroutine is created **only during an active upload request**
2. SHA256 calculation is **very fast** (microseconds for typical files)
3. The goroutine completes and **exits immediately** after `wg.Done()`
4. If you capture pprof when **no upload is happening**, the goroutine doesn't exist

**Timeline:**
```
Request arrives → Goroutine created → Checksum calculated (1ms) → Goroutine exits → Request completes
                                                                    ↑
                                                    If you capture pprof here, goroutine is gone!
```

## Why Redis Calls Don't Appear as Separate Goroutines

Looking at the code:

```go
// Line 173: Redis lock acquisition
acquired, err := h.redis.AcquireLock(ctx, lockKey, 30*time.Second)

// Line 242: Redis metadata storage  
err := h.redis.StoreFileMetadata(ctx, metadata)
```

**Why they don't show up:**
1. Redis calls are **synchronous** - they block the calling goroutine
2. They happen **inside the request handler goroutine** (not in separate goroutines)
3. The Redis client uses the **same goroutine** that handles the HTTP request
4. When blocked on Redis I/O, the goroutine shows as `[IO wait]` or `[syscall]`, but it's still the **request handler goroutine**

**What you'd see during an active request:**
```
goroutine 42 [IO wait]:
    net.(*conn).Read(...)           ← Waiting for Redis response
    github.com/redis/go-redis/v9... ← Redis client code
    control-plane/internal/redis... ← Your Redis wrapper
    control-plane/internal/handlers... ← Your handler
    main.main.func1()               ← HTTP request handler
```

But this only appears **while the request is actively waiting for Redis**.

## When You WOULD See These Goroutines

### Scenario 1: During Active Upload Request
If you capture pprof **while an upload is in progress**:

```bash
# Terminal 1: Start upload (large file, takes time)
curl -X POST http://localhost:8080/upload -F "file=@largefile.zip" ...

# Terminal 2: Immediately capture pprof (while upload is happening)
curl 'http://localhost:8080/debug/pprof/goroutine?debug=2'
```

You'd see:
- Request handler goroutine (parsing form, reading file)
- Checksum calculation goroutine (if still running)
- File write goroutine (if still writing)
- Redis call goroutine (blocked waiting for Redis response)

### Scenario 2: Slow Redis Connection
If Redis is slow or network is laggy:

```bash
# Simulate slow Redis
redis-cli CONFIG SET slowlog-log-slower-than 0

# Make request
curl -X POST http://localhost:8080/upload ...

# Capture pprof while waiting
curl 'http://localhost:8080/debug/pprof/goroutine?debug=2'
```

You'd see the request handler goroutine blocked on Redis I/O.

### Scenario 3: Long-Running Operations
Operations that take time will show up:

```go
// If checksum took 5 seconds (very large file)
go func() {
    time.Sleep(5 * time.Second)  // Simulate slow operation
    hash := sha256.Sum256(fileData)
}()
```

If you capture during those 5 seconds, you'd see it.

## How to See These Goroutines in Action

### Method 1: Capture During Active Request

```bash
# Terminal 1: Start a slow upload
curl -X POST http://localhost:8080/upload \
  -F "file=@very-large-file.zip" \
  -F "filename=test" \
  -F "version=1.0.0" &

# Terminal 2: Immediately capture (within 1 second)
sleep 0.5
curl 'http://localhost:8080/debug/pprof/goroutine?debug=2' > dump.txt
```

### Method 2: Add Delays for Testing

Temporarily modify the code to add delays:

```go
// In processFileUpload, add delay
go func() {
    defer wg.Done()
    time.Sleep(2 * time.Second)  // Give time to capture
    hash := sha256.Sum256(fileData)
    checksum = hex.EncodeToString(hash[:])
}()
```

### Method 3: Use Trace Instead of pprof

For understanding goroutine lifecycle:

```go
import _ "net/http/pprof"
import "runtime/trace"

// In your handler
f, _ := os.Create("trace.out")
trace.Start(f)
defer trace.Stop()

// Your code here
```

Then analyze with:
```bash
go tool trace trace.out
```

## Summary

| Operation | Why Not Visible | When It Would Be Visible |
|-----------|----------------|--------------------------|
| **Checksum calc** | Completes in microseconds, goroutine exits immediately | During active upload (if you capture fast enough) |
| **Redis calls** | Synchronous, happens in request handler goroutine | While request is waiting for Redis response |
| **File writes** | Usually fast, goroutine completes quickly | During large file uploads |
| **HTTP handlers** | Created per-request, complete quickly | During active request processing |

**Key Takeaway:** pprof shows what's happening **right now**. Fast operations that complete before you capture won't appear. To see them, you need to capture **during** the operation, not after.
