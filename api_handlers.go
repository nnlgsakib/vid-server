package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

// getLatestVideoHandler returns the most recently uploaded video
func (s *Server) getLatestVideoHandler(c *gin.Context) {
	video, exists := s.db.GetLatestVideo()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "no videos found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"video":   video,
	})
}

// getAllVideosHandler returns all videos with optional pagination
func (s *Server) getAllVideosHandler(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	allVideos := s.db.GetAllVideos()
	
	// Calculate pagination
	start := (page - 1) * limit
	if start >= len(allVideos) {
		start = len(allVideos)
	}
	
	end := start + limit
	if end > len(allVideos) {
		end = len(allVideos)
	}

	paginatedVideos := allVideos[start:end]

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"videos":  paginatedVideos,
		"total":   len(allVideos),
		"page":    page,
		"limit":   limit,
	})
}

// deleteVideoHandler deletes a video by ID
func (s *Server) deleteVideoHandler(c *gin.Context) {
	videoID := c.Param("id")
	
	video, exists := s.db.GetVideoByID(videoID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	// Remove from database
	deleted := s.db.DeleteVideo(videoID)
	if !deleted {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete video from database"})
		return
	}

	// Remove file from disk
	filePath := s.getFilePath(videoID, video.Name)
	if err := os.Remove(filePath); err != nil {
		s.logger.Error().Err(err).Str("filepath", filePath).Msg("failed to delete video file from disk")
		// Don't return error here since the video is already removed from DB
	}

	s.logger.Info().
		Str("video_id", videoID).
		Str("filename", video.Name).
		Msg("video deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "video deleted successfully",
	})
}

// getFilePath constructs the file path for a video
func (s *Server) getFilePath(videoID, filename string) string {
	return filepath.Join(s.config.StoragePath, videoID+"_"+filename)
}