package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// Config holds server configuration
type Config struct {
	ServerPort       string
	StoragePath      string
	MaxFileSize      int64
	EnableLogging    bool
}

// Video represents a video entry in our system
type Video struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	URL         string    `json:"url"`
}

// InMemoryDB represents our optimized in-memory database
type InMemoryDB struct {
	videos map[string]*Video
	mutex  sync.RWMutex
	
	// Indexes for faster lookups
	nameIndex map[string]string // name -> id
	latestID  string           // most recently added video ID
}

// NewInMemoryDB creates a new instance of the in-memory database
func NewInMemoryDB() *InMemoryDB {
	return &InMemoryDB{
		videos:    make(map[string]*Video),
		nameIndex: make(map[string]string),
	}
}

// AddVideo adds a video to the database
func (db *InMemoryDB) AddVideo(v *Video) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	
	db.videos[v.ID] = v
	db.nameIndex[v.Name] = v.ID
	db.latestID = v.ID
}

// GetVideoByID retrieves a video by its ID
func (db *InMemoryDB) GetVideoByID(id string) (*Video, bool) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	
	video, exists := db.videos[id]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent concurrent modification
	videoCopy := *video
	return &videoCopy, true
}

// GetVideoByName retrieves a video by its name
func (db *InMemoryDB) GetVideoByName(name string) (*Video, bool) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	
	id, exists := db.nameIndex[name]
	if !exists {
		return nil, false
	}
	
	video, exists := db.videos[id]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent concurrent modification
	videoCopy := *video
	return &videoCopy, true
}

// GetLatestVideo returns the most recently added video
func (db *InMemoryDB) GetLatestVideo() (*Video, bool) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	
	if db.latestID == "" {
		return nil, false
	}
	
	video, exists := db.videos[db.latestID]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent concurrent modification
	videoCopy := *video
	return &videoCopy, true
}

// DeleteVideo removes a video from the database
func (db *InMemoryDB) DeleteVideo(id string) bool {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	
	video, exists := db.videos[id]
	if !exists {
		return false
	}
	
	delete(db.videos, id)
	delete(db.nameIndex, video.Name)
	
	// Update latestID if this was the latest video
	if db.latestID == id {
		// Find the new latest video
		db.latestID = ""
		for vidID, vid := range db.videos {
			if db.latestID == "" || vid.CreatedAt.After(db.videos[db.latestID].CreatedAt) {
				db.latestID = vidID
			}
		}
	}
	
	return true
}

// GetAllVideos returns all videos
func (db *InMemoryDB) GetAllVideos() []*Video {
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	
	videos := make([]*Video, 0, len(db.videos))
	for _, video := range db.videos {
		// Return copies to prevent concurrent modification
		videoCopy := *video
		videos = append(videos, &videoCopy)
	}
	
	return videos
}

// Server represents the main server
type Server struct {
	config       *Config
	db           *InMemoryDB
	webhookMgr   *WebhookManager
	router       *gin.Engine
	logger       zerolog.Logger
}

// NewServer creates a new server instance
func NewServer(config *Config) *Server {
	// Initialize logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	if config.EnableLogging {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	server := &Server{
		config:     config,
		db:         NewInMemoryDB(),
		webhookMgr: NewWebhookManager(),
		logger:     logger.With().Str("component", "server").Logger(),
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	gin.SetMode(gin.ReleaseMode)
	s.router = gin.New()

	// Middleware
	s.router.Use(gin.Recovery())
	s.router.Use(s.loggingMiddleware())

	// Health check
	s.router.GET("/health", s.healthHandler)

	// Video endpoints
	videoGroup := s.router.Group("/api/videos")
	{
		videoGroup.POST("", s.uploadVideoHandler)
		videoGroup.GET("/:id", s.downloadVideoHandler)
		videoGroup.DELETE("/:id", s.deleteVideoHandler)
		videoGroup.GET("/latest", s.getLatestVideoHandler)
		videoGroup.GET("", s.getAllVideosHandler)
	}

	// Webhook endpoints
	webhookGroup := s.router.Group("/api/webhooks")
	{
		webhookGroup.POST("", s.addWebhookHandler)
		webhookGroup.GET("", s.getWebhooksHandler)
		webhookGroup.DELETE("", s.removeWebhookHandler)
	}
}

// loggingMiddleware logs incoming requests
func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		c.Next()
		
		duration := time.Since(start)
		
		s.logger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("duration", duration).
			Msg("request completed")
	}
}

// healthHandler returns server health status
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// Run starts the HTTP server
func (s *Server) Run() error {
	s.logger.Info().Str("port", s.config.ServerPort).Msg("starting server")
	
	srv := &http.Server{
		Addr:    ":" + s.config.ServerPort,
		Handler: s.router,
	}
	
	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		<-sigChan
		
		s.logger.Info().Msg("shutting down server...")
		
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		if err := srv.Shutdown(ctx); err != nil {
			s.logger.Error().Err(err).Msg("server shutdown error")
		}
	}()
	
	return srv.ListenAndServe()
}

func main() {
	config := LoadConfig()

	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(config.StoragePath, 0755); err != nil {
		log.Fatal(fmt.Sprintf("failed to create storage directory: %v", err))
	}

	server := NewServer(config)

	if err := server.Run(); err != nil && err != http.ErrServerClosed {
		log.Fatal(fmt.Sprintf("server error: %v", err))
	}
}