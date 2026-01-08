# Cloudflare Control Plane

A Go-based control plane for managing file versions with Redis metadata storage and local file storage (MinIO-ready).

## Architecture

- **Upload Service**: HTTP server for uploading files with metadata (version, filename)
- **Consumer Service**: HTTP server for retrieving files (latest or specific version)
- **Redis**: Central metadata store (checksum, version, file link)
- **Local Storage**: File storage in local filesystem (designed to support MinIO in future)

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.21+ (for local development)

### Running with Docker Compose

1. Start all services:
```bash
docker compose up -d
```

2. Check service status:
```bash
docker compose ps
```

3. View logs:
```bash
docker compose logs -f
```

### Scaling Consumers

To run multiple consumer instances:

```bash
docker compose up -d --scale consumer-server=3
```

Note: You'll need to configure a load balancer (nginx/traefik) in front of consumers when scaling, as Docker Compose will assign random ports.

## API Endpoints

### Upload Service (Port 8080)

- `POST /upload` - Upload a file
  - Form data: `filename`, `version`, `file`
  - Returns: Upload confirmation with metadata

- `GET /health` - Health check

### Consumer Service (Port 8081)

- `GET /file/{filename}` - Get latest version of file
- `GET /file/{filename}/version/{version}` - Get specific version
- `GET /file/{filename}/info` - Get file metadata (latest version)
- `GET /health` - Health check

## Web UI

Access the upload UI at `http://localhost:8080` (when served via upload server) or open `web/index.html` directly in a browser.

## Configuration

Configuration is done via environment variables:

- `REDIS_HOST` - Redis host (default: `redis`)
- `REDIS_PORT` - Redis port (default: `6379`)
- `REDIS_PASSWORD` - Redis password (default: empty)
- `STORAGE_TYPE` - Storage type: `local` or `minio` (default: `local`)
- `STORAGE_PATH` - File storage path (default: `/app/files`)
- `UPLOAD_PORT` - Upload service port (default: `8080`)
- `CONSUMER_PORT` - Consumer service port (default: `8081`)

## Local Development

### Build

```bash
go build -o upload-server ./cmd/upload-server
go build -o consumer-server ./cmd/consumer-server
```

### Run

1. Start Redis:
```bash
docker run -d -p 6379:6379 --name redis redis:alpine
```

2. Run upload server:
```bash
export REDIS_HOST=localhost
export REDIS_PORT=6379
export STORAGE_PATH=./files
./upload-server
```

3. Run consumer server:
```bash
export REDIS_HOST=localhost
export REDIS_PORT=6379
export STORAGE_PATH=./files
./consumer-server
```

## Redis Schema

- `file:{filename}:{version}` - Hash with metadata (checksum, version, filepath, uploaded_at, size)
- `file:{filename}:versions` - Sorted Set with versions (timestamp as score)
- `file:{filename}:latest` - String with latest version number

## File Storage

Files are stored in the structure: `files/{filename}/{version}`

## Testing

A Python test script is provided to test all endpoints:

```bash
# Install Python dependencies
pip install -r requirements.txt

# Run tests (make sure services are running)
python3 test_control_plane.py
```

The test script will:
- Test health check endpoints
- Upload files with different versions
- Retrieve latest and specific versions
- Verify file metadata
- Test error handling (nonexistent files)

## Future Enhancements

- MinIO/S3 storage support
- Authentication/authorization
- File deletion endpoints
- Version listing endpoints
- Webhook notifications
