package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bitcoin-price-streamer/internal/models"
	"bitcoin-price-streamer/internal/storage"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewPriceService(t *testing.T) {
	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)

	service := NewPriceService(storage, logger)

	assert.NotNil(t, service)
	assert.Equal(t, storage, service.storage)
	assert.Equal(t, logger, service.logger)
	assert.NotNil(t, service.clients)
	assert.NotNil(t, service.httpClient)
	assert.Equal(t, "https://data-api.coindesk.com/asset/v1/top/list", service.apiURL)
	assert.Equal(t, 50, service.bufferSize) // Default value
}

func TestSubscribeAndUnsubscribe(t *testing.T) {
	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)
	service := NewPriceService(storage, logger)

	// Subscribe
	clientChan := service.Subscribe()
	assert.NotNil(t, clientChan)
	assert.Len(t, service.clients, 1)

	// Unsubscribe
	service.Unsubscribe(clientChan)
	assert.Len(t, service.clients, 0)

	// Verify channel is closed
	_, ok := <-clientChan
	assert.False(t, ok, "Channel should be closed after unsubscribe")
}

func TestBroadcastPrice(t *testing.T) {
	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)
	service := NewPriceService(storage, logger)

	// Subscribe two clients
	client1 := service.Subscribe()
	client2 := service.Subscribe()

	// Create a price update
	price := models.PriceUpdate{
		Timestamp: time.Now(),
		Price:     50000.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}

	// Broadcast in a goroutine to avoid blocking
	go service.broadcastPrice(price)

	// Receive from both clients
	select {
	case received1 := <-client1:
		assert.Equal(t, price.Price, received1.Price)
		assert.Equal(t, price.Symbol, received1.Symbol)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for client1 to receive price")
	}

	select {
	case received2 := <-client2:
		assert.Equal(t, price.Price, received2.Price)
		assert.Equal(t, price.Symbol, received2.Symbol)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for client2 to receive price")
	}

	// Cleanup
	service.Unsubscribe(client1)
	service.Unsubscribe(client2)
}

func TestBroadcastPriceWithBlockedClient(t *testing.T) {
	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)
	service := NewPriceService(storage, logger)

	// Subscribe with small buffer
	clientChan := make(chan models.PriceUpdate, 1)
	service.clientsMux.Lock()
	service.clients[clientChan] = true
	service.clientsMux.Unlock()

	// Fill the buffer
	price1 := models.PriceUpdate{Price: 50000.0, Timestamp: time.Now()}
	clientChan <- price1

	// Try to broadcast another price (should remove blocked client)
	price2 := models.PriceUpdate{Price: 51000.0, Timestamp: time.Now()}
	service.broadcastPrice(price2)

	// Client should be removed
	service.clientsMux.RLock()
	_, exists := service.clients[clientChan]
	service.clientsMux.RUnlock()
	assert.False(t, exists, "Blocked client should be removed")
}

func TestFetchBitcoinPrice(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer server.Close()

	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)
	service := NewPriceService(storage, logger)
	service.apiURL = server.URL // Use mock server

	price, err := service.fetchBitcoinPrice()

	assert.NoError(t, err)
	assert.NotNil(t, price)
	assert.Equal(t, "BTC", price.Symbol)
	assert.Equal(t, "Bitcoin", price.Name)
	assert.Equal(t, 50000.0, price.Price)
}

func TestFetchBitcoinPriceNotFound(t *testing.T) {
	// Create a mock server that doesn't return Bitcoin
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
						Symbol:               "ETH",
						Name:                 "Ethereum",
						PriceUSD:             3000.0,
						PriceUSDLastUpdateTS: time.Now().Unix(),
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)
	service := NewPriceService(storage, logger)
	service.apiURL = server.URL

	price, err := service.fetchBitcoinPrice()

	assert.Error(t, err)
	assert.Nil(t, price)
	assert.Contains(t, err.Error(), "bitcoin data not found")
}

func TestFetchBitcoinPriceAPIError(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)
	service := NewPriceService(storage, logger)
	service.apiURL = server.URL

	price, err := service.fetchBitcoinPrice()

	assert.Error(t, err)
	assert.Nil(t, price)
	assert.Contains(t, err.Error(), "status code: 500")
}

func TestFetchBitcoinPriceInvalidJSON(t *testing.T) {
	// Create a mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)
	service := NewPriceService(storage, logger)
	service.apiURL = server.URL

	price, err := service.fetchBitcoinPrice()

	assert.Error(t, err)
	assert.Nil(t, price)
	assert.Contains(t, err.Error(), "failed to decode")
}

func TestFetchAndBroadcastPrice(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer server.Close()

	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)
	service := NewPriceService(storage, logger)
	service.apiURL = server.URL

	// Subscribe a client
	clientChan := service.Subscribe()
	defer service.Unsubscribe(clientChan)

	// Fetch and broadcast
	service.fetchAndBroadcastPrice()

	// Check that price was stored
	latest, exists := storage.GetLatest()
	assert.True(t, exists)
	assert.Equal(t, 50000.0, latest.Price)

	// Check that client received the price
	select {
	case received := <-clientChan:
		assert.Equal(t, 50000.0, received.Price)
		assert.Equal(t, "BTC", received.Symbol)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for client to receive price")
	}
}

func TestStartPolling(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer server.Close()

	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)
	service := NewPriceService(storage, logger)
	service.apiURL = server.URL

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start polling
	go service.StartPolling(ctx)

	// Wait for context to be cancelled
	<-ctx.Done()

	// Check that at least one price was fetched
	latest, exists := storage.GetLatest()
	assert.True(t, exists, "At least one price should be fetched")
	assert.Equal(t, 50000.0, latest.Price)
}

func TestGetStorage(t *testing.T) {
	logger := logrus.New()
	storage := storage.NewPriceStorage(context.Background(), 100, logger)
	service := NewPriceService(storage, logger)

	retrievedStorage := service.GetStorage()
	assert.Equal(t, storage, retrievedStorage)
}
