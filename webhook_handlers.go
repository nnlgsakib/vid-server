package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// addWebhookHandler adds a new webhook URL for an event
func (s *Server) addWebhookHandler(c *gin.Context) {
	var req struct {
		Event string `json:"event" binding:"required"`
		URL   string `json:"url" binding:"required,url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.webhookMgr.AddWebhook(req.Event, req.URL)

	s.logger.Info().
		Str("event", req.Event).
		Str("url", req.URL).
		Msg("webhook added")

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "webhook added successfully",
		"event":   req.Event,
		"url":     req.URL,
	})
}

// getWebhooksHandler returns all registered webhooks
func (s *Server) getWebhooksHandler(c *gin.Context) {
	event := c.Query("event")

	if event != "" {
		// Return webhooks for specific event
		urls := s.webhookMgr.GetWebhooks(event)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"event":   event,
			"urls":    urls,
		})
	} else {
		// Return all webhooks
		allWebhooks := s.webhookMgr.GetAllWebhooks()
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"webhooks": allWebhooks,
		})
	}
}

// removeWebhookHandler removes a webhook URL for an event
func (s *Server) removeWebhookHandler(c *gin.Context) {
	var req struct {
		Event string `json:"event" binding:"required"`
		URL   string `json:"url" binding:"required,url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.webhookMgr.RemoveWebhook(req.Event, req.URL)

	s.logger.Info().
		Str("event", req.Event).
		Str("url", req.URL).
		Msg("webhook removed")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "webhook removed successfully",
		"event":   req.Event,
		"url":     req.URL,
	})
}

// testWebhookHandler sends a test webhook call to a specified URL
func (s *Server) testWebhookHandler(c *gin.Context) {
	var req struct {
		URL     string `json:"url" binding:"required,url"`
		Event   string `json:"event"`
		VideoID string `json:"videoId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default event if not provided
	if req.Event == "" {
		req.Event = "video.uploaded"
	}

	// Generate test video ID if not provided
	if req.VideoID == "" {
		req.VideoID = uuid.New().String()
	}

	// Create test video payload
	testVideo := &Video{
		ID:          req.VideoID,
		Name:        "test_video.mp4",
		Size:        12345678,
		ContentType: "video/mp4",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		URL:         fmt.Sprintf("/api/videos/%s", req.VideoID),
	}

	// Send test webhook
	payload := gin.H{
		"video":     testVideo,
		"event":     req.Event,
		"timestamp": time.Now().Unix(),
		"is_test":   true,
		"test_mode": true,
	}

	s.logger.Info().
		Str("url", req.URL).
		Str("event", req.Event).
		Msg("sending test webhook")

	// Send webhook in a goroutine (async)
	go func() {
		_ = s.webhookMgr.SendDirectWebhook(req.URL, payload)
	}()

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "test webhook sent successfully",
		"url":       req.URL,
		"event":     req.Event,
		"video_id":  req.VideoID,
		"timestamp": time.Now().Unix(),
	})
}
