package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"bitcoin-price-streamer/internal/handlers"
	"bitcoin-price-streamer/internal/models"
	"bitcoin-price-streamer/internal/service"
	"bitcoin-price-streamer/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationFlow(t *testing.T) {
	// Create mock API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := models.CoinDeskResponse{
			Data: struct {
				Stats struct {
					Page        int `json:"PAGE"`
					PageSize    int `json:"PAGE_SIZE"`
					TotalAssets int `json:"TOTAL_ASSETS"`
				} `json:"STATS"`
				List []models.AssetData `json:"LIST"`
			}{
				List: []models.AssetData{
					{
						Symbol:               "BTC",
						Name:                 "Bitcoin",
						PriceUSD:             50000.0,
						PriceUSDLastUpdateTS: time.Now().Unix(),
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Set up the application
	logger := logrus.New()
	ctx := context.Background()

	// Create storage
	storage := storage.NewPriceStorage(ctx, 100, logger)

	// Create service with mock API
	priceService := service.NewPriceService(storage, logger)
	priceService.SetAPIURL(mockServer.URL)

	// Create handlers
	handlers := handlers.NewHandlers(priceService, logger)

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.SetupRoutes(router)

	// Test 1: Current price endpoint
	t.Run("Current Price Endpoint", func(t *testing.T) {
		// Start polling to populate storage
		pollCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		go priceService.StartPolling(pollCtx)
		<-pollCtx.Done()
		cancel()

		// Test current price endpoint
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/price/current", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.PriceUpdate
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "BTC", response.Symbol)
		assert.Equal(t, 50000.0, response.Price)
	})

	// Test 2: Price history endpoint
	t.Run("Price History Endpoint", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/price/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		updates, ok := response["updates"].([]interface{})
		assert.True(t, ok)
		assert.Greater(t, len(updates), 0)
	})

	// Test 3: SSE endpoint
	t.Run("SSE Endpoint", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/price/stream", nil)

		// Set up SSE headers
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")

		// Start the request in a goroutine
		go router.ServeHTTP(w, req)

		// Wait a bit for the connection to establish
		time.Sleep(50 * time.Millisecond)

		// The response should have SSE headers
		assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	})
}

func TestStaticFileServing(t *testing.T) {
	// Set static path to the parent directory for tests
	os.Setenv("STATIC_PATH", "../static")
	defer os.Unsetenv("STATIC_PATH")

	// Set up the application
	logger := logrus.New()
	ctx := context.Background()

	// Create storage
	storage := storage.NewPriceStorage(ctx, 10, logger)

	// Create service
	priceService := service.NewPriceService(storage, logger)

	// Create handlers
	h := handlers.NewHandlers(priceService, logger)

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h.SetupRoutes(router)

	// Test static file serving
	t.Run("Index Page", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Bitcoin Price Streamer")
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	})

	t.Run("Static File Not Found", func(t *testing.T) {
		// Test with invalid static path
		os.Setenv("STATIC_PATH", "/nonexistent/path")
		defer os.Setenv("STATIC_PATH", "../static")

		// Create new handlers with invalid path
		newHandlers := handlers.NewHandlers(priceService, logger)
		router := gin.New()
		newHandlers.SetupRoutes(router)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "404 page not found")
	})
}

func TestIntegrationWithRealData(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()

	// Create storage
	storage := storage.NewPriceStorage(ctx, 10, logger)

	// Create service (will use real API)
	priceService := service.NewPriceService(storage, logger)

	// Create handlers
	handlers := handlers.NewHandlers(priceService, logger)

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.SetupRoutes(router)

	// Test with real API
	t.Run("Real API Integration", func(t *testing.T) {
		// Start polling with longer timeout to allow for API response
		pollCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		go priceService.StartPolling(pollCtx)

		// Wait for at least one successful API call
		var latest models.PriceUpdate
		var exists bool
		for i := 0; i < 30; i++ { // Try for up to 3 seconds
			time.Sleep(100 * time.Millisecond)
			latest, exists = storage.GetLatest()
			if exists {
				break
			}
		}

		cancel()

		// Verify we got data from the real API
		if !exists {
			t.Fatal("Failed to fetch data from real API within timeout period")
		}

		// Validate the data structure
		assert.Equal(t, "BTC", latest.Symbol)
		assert.Greater(t, latest.Price, 0.0)
		assert.NotZero(t, latest.Timestamp)
		assert.NotEmpty(t, latest.Name)

		// Test current price endpoint
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/price/current", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.PriceUpdate
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "BTC", response.Symbol)
		assert.Greater(t, response.Price, 0.0)
		assert.Equal(t, latest.Price, response.Price)

		// Test price history endpoint
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/price/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var historyResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &historyResponse)
		require.NoError(t, err)

		updates, ok := historyResponse["updates"].([]interface{})
		assert.True(t, ok)
		assert.Greater(t, len(updates), 0)

		// Test SSE endpoint with real data
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/price/stream", nil)
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")

		// Start the request in a goroutine
		go router.ServeHTTP(w, req)

		// Wait a bit for the connection to establish
		time.Sleep(100 * time.Millisecond)

		// The response should have SSE headers
		assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	})
}

func TestIntegrationErrorHandling(t *testing.T) {
	// Create mock server that returns errors
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer mockServer.Close()

	logger := logrus.New()
	ctx := context.Background()

	// Create storage
	storage := storage.NewPriceStorage(ctx, 10, logger)

	// Create service with failing API
	priceService := service.NewPriceService(storage, logger)
	priceService.SetAPIURL(mockServer.URL)

	// Create handlers
	handlers := handlers.NewHandlers(priceService, logger)

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.SetupRoutes(router)

	// Test error handling
	t.Run("Error Handling", func(t *testing.T) {
		// Start polling briefly
		pollCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		go priceService.StartPolling(pollCtx)
		<-pollCtx.Done()
		cancel()

		// Test current price endpoint (should return 404 since no data)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/price/current", nil)
		router.ServeHTTP(w, req)

		// Should return 404 since no price data is available
		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "No price data available")
	})
}
