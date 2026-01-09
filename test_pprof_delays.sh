#!/bin/bash

echo "=== Testing pprof with Delays ==="
echo ""
echo "This script demonstrates how to see checksum and Redis goroutines in pprof"
echo ""

# Create a test file
echo "Creating test file..."
echo "test content for pprof demonstration" > /tmp/test-upload.txt

echo ""
echo "Step 1: Start upload in background (will take ~7 seconds with delays)"
echo "Command:"
echo "  curl -X POST http://localhost:8080/upload \\"
echo "    -F \"file=@/tmp/test-upload.txt\" \\"
echo "    -F \"filename=pprof-test\" \\"
echo "    -F \"version=1.0.0\" &"
echo ""

# Start upload
UPLOAD_PID=$(curl -X POST http://localhost:8080/upload \
  -F "file=@/tmp/test-upload.txt" \
  -F "filename=pprof-test" \
  -F "version=1.0.0" 2>&1 > /tmp/upload-response.json & echo $!)

echo "Upload started (PID: $UPLOAD_PID)"
echo ""

echo "Step 2: Wait 1 second for request to start processing..."
sleep 1

echo ""
echo "Step 3: Capture pprof dump while upload is processing..."
curl -s 'http://localhost:8080/debug/pprof/goroutine?debug=2' > /tmp/pprof-dump.txt

echo "Dump saved to /tmp/pprof-dump.txt"
echo ""

echo "Step 4: Search for checksum and Redis goroutines..."
echo ""
echo "=== Checksum Goroutine (should show sleep) ==="
grep -A 15 "processFileUpload\|checksum\|sha256" /tmp/pprof-dump.txt | head -20 || echo "Not found (request may have completed)"

echo ""
echo "=== Redis Lock Goroutine (should show blocked on Redis) ==="
grep -A 15 "AcquireLock\|redis" /tmp/pprof-dump.txt | head -20 || echo "Not found (request may have completed)"

echo ""
echo "=== Redis Store Goroutine (should show blocked on Redis) ==="
grep -A 15 "StoreFileMetadata\|storeMetadata" /tmp/pprof-dump.txt | head -20 || echo "Not found (request may have completed)"

echo ""
echo "=== All Goroutines Summary ==="
grep "^goroutine" /tmp/pprof-dump.txt | head -10

echo ""
echo "Full dump available at: /tmp/pprof-dump.txt"
echo ""
