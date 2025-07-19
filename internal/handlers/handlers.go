package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"bitcoin-price-streamer/internal/models"
	"bitcoin-price-streamer/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// Handlers manages HTTP request handlers
type Handlers struct {
	priceService *service.PriceService
	logger       *logrus.Logger
	upgrader     websocket.Upgrader
}

// NewHandlers creates new HTTP handlers
func NewHandlers(priceService *service.PriceService, logger *logrus.Logger) *Handlers {
	return &Handlers{
		priceService: priceService,
		logger:       logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// SetupRoutes configures all the routes for the application
func (h *Handlers) SetupRoutes(router *gin.Engine) {
	// API routes
	api := router.Group("/api")
	{
		api.GET("/price/stream", h.handleSSE)
		api.GET("/price/current", h.handleCurrentPrice)
		api.GET("/price/history", h.handlePriceHistory)
		api.GET("/ws", h.handleWebSocket)
	}

	// Serve the main page
	router.GET("/", h.handleIndex)
}

// handleIndex serves the main HTML page
func (h *Handlers) handleIndex(c *gin.Context) {
	c.File("./static/index.html")
}

// handleSSE handles Server-Sent Events for real-time price streaming
func (h *Handlers) handleSSE(c *gin.Context) {
	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	// Get the 'since' parameter for missed updates
	sinceParam := c.Query("since")
	var since time.Time
	if sinceParam != "" {
		if timestamp, err := strconv.ParseInt(sinceParam, 10, 64); err == nil {
			since = time.Unix(timestamp, 0)
		}
	}

	// Send missed updates if 'since' parameter is provided
	if !since.IsZero() {
		storage := h.priceService.GetStorage()
		missedUpdates := storage.GetUpdatesSince(since)

		for _, update := range missedUpdates {
			data, _ := json.Marshal(update)
			c.SSEvent("price", string(data))
		}
	}

	// Subscribe to real-time updates
	clientChan := h.priceService.Subscribe()
	defer h.priceService.Unsubscribe(clientChan)

	// Create a channel to detect client disconnection
	notify := c.Writer.CloseNotify()

	for {
		select {
		case price := <-clientChan:
			data, err := json.Marshal(price)
			if err != nil {
				h.logger.Errorf("Failed to marshal price update: %v", err)
				continue
			}
			c.SSEvent("price", string(data))
			c.Writer.Flush()
		case <-notify:
			h.logger.Info("Client disconnected from SSE")
			return
		case <-c.Request.Context().Done():
			h.logger.Info("Request context cancelled")
			return
		}
	}
}

// handleCurrentPrice returns the current Bitcoin price
func (h *Handlers) handleCurrentPrice(c *gin.Context) {
	storage := h.priceService.GetStorage()
	price, exists := storage.GetLatest()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "No price data available"})
		return
	}

	c.JSON(http.StatusOK, price)
}

// handlePriceHistory returns price history with optional filtering
func (h *Handlers) handlePriceHistory(c *gin.Context) {
	storage := h.priceService.GetStorage()

	// Get query parameters
	sinceParam := c.Query("since")
	limitParam := c.Query("limit")

	var since time.Time
	if sinceParam != "" {
		if timestamp, err := strconv.ParseInt(sinceParam, 10, 64); err == nil {
			since = time.Unix(timestamp, 0)
		}
	}

	limit := 100 // default limit
	if limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
			limit = l
		}
	}

	var updates []models.PriceUpdate
	if !since.IsZero() {
		updates = storage.GetUpdatesSince(since)
	} else {
		updates = storage.GetAllUpdates()
	}

	// Apply limit
	if len(updates) > limit {
		updates = updates[len(updates)-limit:]
	}

	c.JSON(http.StatusOK, gin.H{
		"updates": updates,
		"count":   len(updates),
	})
}

// handleWebSocket handles WebSocket connections for real-time price updates
func (h *Handlers) handleWebSocket(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Errorf("Failed to upgrade connection to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	h.logger.Info("New WebSocket connection established")

	// Subscribe to price updates
	clientChan := h.priceService.Subscribe()
	defer h.priceService.Unsubscribe(clientChan)

	// Handle WebSocket messages (for future features)
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				h.logger.Debugf("WebSocket read error: %v", err)
				return
			}
			h.logger.Debugf("Received WebSocket message: %s", string(message))
		}
	}()

	// Send price updates to WebSocket client
	for {
		select {
		case price := <-clientChan:
			data, err := json.Marshal(price)
			if err != nil {
				h.logger.Errorf("Failed to marshal price update: %v", err)
				continue
			}

			err = conn.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				h.logger.Errorf("Failed to send WebSocket message: %v", err)
				return
			}
		case <-c.Request.Context().Done():
			h.logger.Info("WebSocket context cancelled")
			return
		}
	}
}
