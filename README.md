# High-Performance Video Server

A high-performance Go-based video server with in-memory database optimized for video storage and retrieval.

## Features

- **High-performance**: Optimized for fast video upload and download
- **In-memory database**: Fast lookups with O(1) average time complexity
- **Range requests**: Supports HTTP range requests for video streaming
- **API endpoints**: RESTful API for managing videos
- **Concurrent-safe**: Thread-safe operations with mutex protection
- **Configurable**: Environment variable based configuration

## API Endpoints

### Upload Video
```
POST /api/videos
Content-Type: multipart/form-data
Body: file=<video_file>
```

### Download Video
```
GET /api/videos/{id}
```

### Get Latest Video
```
GET /api/videos/latest
```

### Get All Videos
```
GET /api/videos?page=1&limit=20
```

### Delete Video
```
DELETE /api/videos/{id}
```

### Health Check
```
GET /health
```

## Configuration

The server can be configured using environment variables:

- `SERVER_PORT`: Port to run the server on (default: 8080)
- `STORAGE_PATH`: Directory to store video files (default: ./storage)
- `MAX_FILE_SIZE`: Maximum file size in bytes (default: 524288000 = 500MB)
- `ENABLE_LOGGING`: Enable request logging (default: true)

## Getting Started

1. Install Go 1.21 or later
2. Clone the repository
3. Run the server:

```bash
cd video-server
go mod tidy
go run .
```

Or build and run:

```bash
go build -o video-server
./video-server
```

## Performance Notes

- The in-memory database provides O(1) average lookup time for video metadata
- Range request support enables efficient video streaming
- Concurrent-safe operations allow for high throughput
- Files are stored on disk while metadata is kept in memory for fast access

## Integration with Python Script

To integrate with your existing Python script, update the upload and download logic to use the new server endpoints instead of Google Drive.