package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// uploadVideoHandler handles video uploads
func (s *Server) uploadVideoHandler(c *gin.Context) {
	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to parse multipart form")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form data"})
		return
	}

	// Get file from form
	files := form.File["file"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file provided"})
		return
	}

	file := files[0]

	// Validate file size
	if file.Size > s.config.MaxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file too large, max size is %d bytes", s.config.MaxFileSize)})
		return
	}

	// Generate unique ID and filename
	videoID := uuid.New().String()
	filename := sanitizeFilename(file.Filename)

	// Determine content type
	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Create file path
	filePath := filepath.Join(s.config.StoragePath, videoID+"_"+filename)

	// Save file to disk
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		s.logger.Error().Err(err).Str("filepath", filePath).Msg("failed to save uploaded file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	// Get file info
	stat, err := os.Stat(filePath)
	if err != nil {
		s.logger.Error().Err(err).Str("filepath", filePath).Msg("failed to get file stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get file info"})
		return
	}

	// Create video record
	video := &Video{
		ID:          videoID,
		Name:        filename,
		Size:        stat.Size(),
		ContentType: contentType,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		URL:         fmt.Sprintf("/api/videos/%s", videoID),
	}

	// Add to database
	s.db.AddVideo(video)

	s.logger.Info().
		Str("video_id", video.ID).
		Str("filename", video.Name).
		Int64("size", video.Size).
		Msg("video uploaded successfully")

	// Trigger webhook for video upload event
	go s.webhookMgr.NotifyWebhooks("video.uploaded", gin.H{
		"video":     video,
		"event":     "video.uploaded",
		"timestamp": time.Now().Unix(),
	})

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"video":   video,
	})
}

// downloadVideoHandler serves video files with range support
func (s *Server) downloadVideoHandler(c *gin.Context) {
	videoID := c.Param("id")

	video, exists := s.db.GetVideoByID(videoID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	filePath := filepath.Join(s.config.StoragePath, videoID+"_"+video.Name)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.logger.Error().Str("filepath", filePath).Msg("video file not found on disk")
		c.JSON(http.StatusNotFound, gin.H{"error": "video file not found"})
		return
	}

	// Handle range requests for streaming
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		s.serveRangeRequest(c, filePath, video)
		return
	}

	// Serve the entire file
	c.Header("Content-Type", video.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", video.Size))
	c.Header("Accept-Ranges", "bytes")

	http.ServeFile(c.Writer, c.Request, filePath)
}

// serveRangeRequest handles HTTP range requests for video streaming
func (s *Server) serveRangeRequest(c *gin.Context, filePath string, video *Video) {
	file, err := os.Open(filePath)
	if err != nil {
		s.logger.Error().Err(err).Str("filepath", filePath).Msg("failed to open video file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
		return
	}
	defer file.Close()

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		s.logger.Error().Err(err).Str("filepath", filePath).Msg("failed to get file stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get file info"})
		return
	}

	// Parse range header
	start, end, err := parseRangeHeader(c.GetHeader("Range"), stat.Size())
	if err != nil {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", stat.Size()))
		c.JSON(http.StatusRequestedRangeNotSatisfiable, gin.H{"error": "invalid range"})
		return
	}

	// Calculate content length
	contentLength := end - start + 1

	// Seek to start position
	if _, err := file.Seek(start, 0); err != nil {
		s.logger.Error().Err(err).Int64("start", start).Msg("failed to seek file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	// Set headers
	c.Header("Content-Type", video.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", contentLength))
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, stat.Size()))
	c.Header("Accept-Ranges", "bytes")

	// Set status code for partial content
	c.Status(http.StatusPartialContent)

	// Stream the content
	if _, err := io.CopyN(c.Writer, file, contentLength); err != nil {
		s.logger.Error().Err(err).Msg("failed to stream file")
		return
	}
}

// parseRangeHeader parses the Range header and returns start and end positions
func parseRangeHeader(rangeHeader string, fileSize int64) (int64, int64, error) {
	if rangeHeader == "" {
		return 0, fileSize - 1, nil
	}

	// Format: "bytes=start-end"
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, fmt.Errorf("invalid range header format")
	}

	rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format")
	}

	var start, end int64
	var err error

	if parts[0] == "" {
		// Suffix-byte-range-spec: -500 means the last 500 bytes
		end, err = parseNumber(parts[1])
		if err != nil {
			return 0, 0, err
		}
		start = fileSize - end
		end = fileSize - 1
	} else if parts[1] == "" {
		// From start to end of file
		start, err = parseNumber(parts[0])
		if err != nil {
			return 0, 0, err
		}
		end = fileSize - 1
	} else {
		// Specific range
		start, err = parseNumber(parts[0])
		if err != nil {
			return 0, 0, err
		}
		end, err = parseNumber(parts[1])
		if err != nil {
			return 0, 0, err
		}
	}

	// Validate range
	if start < 0 || start >= fileSize || end < start || end >= fileSize {
		return 0, 0, fmt.Errorf("range out of bounds")
	}

	return start, end, nil
}

// parseNumber parses a number from string
func parseNumber(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// directDownloadHandler serves video files as direct downloads with .mp4 extension
func (s *Server) directDownloadHandler(c *gin.Context) {
	// Get video ID from URL parameter
	videoID := c.Param("id")
	s.logger.Info().Str("video_id", videoID).Msg("direct download requested")

	// Look up video in database
	video, exists := s.db.GetVideoByID(videoID)
	if !exists {
		s.logger.Error().Str("video_id", videoID).Msg("video not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	// Construct file path
	filePath := filepath.Join(s.config.StoragePath, videoID+"_"+video.Name)

	// Check if file exists on disk
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.logger.Error().Str("filepath", filePath).Msg("video file not found on disk")
		c.JSON(http.StatusNotFound, gin.H{"error": "video file not found"})
		return
	}

	// Get file info for Content-Length
	stat, err := os.Stat(filePath)
	if err != nil {
		s.logger.Error().Err(err).Str("filepath", filePath).Msg("failed to get file stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get file info"})
		return
	}

	// Set headers for direct download
	c.Header("Content-Type", "video/mp4")
	c.Header("Content-Length", fmt.Sprintf("%d", stat.Size()))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.mp4\"", videoID))
	c.Header("Accept-Ranges", "bytes")

	// Serve the file
	c.Status(http.StatusOK)
	http.ServeFile(c.Writer, c.Request, filePath)
}

// sanitizeFilename sanitizes a filename to prevent path traversal
func sanitizeFilename(filename string) string {
	// Remove any path separators to prevent directory traversal
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Limit length to prevent abuse
	if len(filename) > 255 {
		ext := filepath.Ext(filename)
		base := filename[:255-len(ext)]
		filename = base + ext
	}

	return filename
}
