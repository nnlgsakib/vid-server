package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
