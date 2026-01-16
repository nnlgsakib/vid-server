package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"
)

// WebhookManager manages webhook subscriptions and notifications
type WebhookManager struct {
	webhooks map[string][]string // event -> urls mapping
	mutex    sync.RWMutex
}

// NewWebhookManager creates a new webhook manager
func NewWebhookManager() *WebhookManager {
	return &WebhookManager{
		webhooks: make(map[string][]string),
	}
}

// AddWebhook adds a webhook URL for a specific event
func (wm *WebhookManager) AddWebhook(event, url string) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	
	// Check if URL already exists for this event
	for _, existingURL := range wm.webhooks[event] {
		if existingURL == url {
			return // URL already exists, don't add duplicate
		}
	}
	
	wm.webhooks[event] = append(wm.webhooks[event], url)
}

// RemoveWebhook removes a webhook URL for a specific event
func (wm *WebhookManager) RemoveWebhook(event, url string) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	
	urls := wm.webhooks[event]
	newUrls := make([]string, 0, len(urls))
	
	for _, existingURL := range urls {
		if existingURL != url {
			newUrls = append(newUrls, existingURL)
		}
	}
	
	wm.webhooks[event] = newUrls
}

// NotifyWebhooks sends notification to all registered webhooks for an event
func (wm *WebhookManager) NotifyWebhooks(event string, payload interface{}) {
	wm.mutex.RLock()
	urls := wm.webhooks[event]
	wm.mutex.RUnlock()
	
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Str("event", event).Msg("failed to marshal webhook payload")
		return
	}
	
	// Send notifications concurrently
	for _, url := range urls {
		go wm.sendWebhookNotification(url, payloadBytes)
	}
}

// sendWebhookNotification sends a single webhook notification
func (wm *WebhookManager) sendWebhookNotification(url string, payload []byte) {
	client := &http.Client{}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("failed to create webhook request")
		return
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("failed to send webhook notification")
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().
			Str("url", url).
			Int("status", resp.StatusCode).
			Msg("webhook notification returned non-success status")
	} else {
		log.Info().Str("url", url).Msg("webhook notification sent successfully")
	}
}

// GetWebhooks returns all registered webhooks for an event
func (wm *WebhookManager) GetWebhooks(event string) []string {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()
	
	urls := make([]string, len(wm.webhooks[event]))
	copy(urls, wm.webhooks[event])
	
	return urls
}

// GetAllWebhooks returns all registered webhooks
func (wm *WebhookManager) GetAllWebhooks() map[string][]string {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()
	
	allWebhooks := make(map[string][]string)
	for event, urls := range wm.webhooks {
		eventUrls := make([]string, len(urls))
		copy(eventUrls, urls)
		allWebhooks[event] = eventUrls
	}
	
	return allWebhooks
}