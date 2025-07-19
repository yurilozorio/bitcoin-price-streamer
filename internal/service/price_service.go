package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"bitcoin-price-streamer/internal/models"
	"bitcoin-price-streamer/internal/storage"
	"bitcoin-price-streamer/internal/utils"

	"github.com/sirupsen/logrus"
)

// PriceService manages Bitcoin price polling and client connections
type PriceService struct {
	storage    *storage.PriceStorage
	logger     *logrus.Logger
	clients    map[chan models.PriceUpdate]bool
	clientsMux sync.RWMutex
	httpClient *http.Client
	apiURL     string
	bufferSize int
}

// NewPriceService creates a new price service
func NewPriceService(storage *storage.PriceStorage, logger *logrus.Logger) *PriceService {
	apiURL := utils.GetEnvString("COINDESK_API_URL", "https://data-api.coindesk.com/asset/v1/top/list")
	bufferSize := utils.GetEnvInt("CLIENT_BUFFER_SIZE", 50)

	return &PriceService{
		storage: storage,
		logger:  logger,
		clients: make(map[chan models.PriceUpdate]bool),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		apiURL:     apiURL,
		bufferSize: bufferSize,
	}
}

// StartPolling starts polling the CoinDesk API for Bitcoin price updates
func (ps *PriceService) StartPolling(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ps.logger.Info("Starting Bitcoin price polling...")

	// Do initial fetch immediately
	ps.fetchAndBroadcastPrice()

	for {
		select {
		case <-ctx.Done():
			ps.logger.Info("Stopping price polling...")
			return
		case <-ticker.C:
			ps.fetchAndBroadcastPrice()
		}
	}
}

// fetchAndBroadcastPrice fetches the latest Bitcoin price and broadcasts to all clients
func (ps *PriceService) fetchAndBroadcastPrice() {
	price, err := ps.fetchBitcoinPrice()
	if err != nil {
		ps.logger.Errorf("Failed to fetch Bitcoin price: %v", err)
		return
	}

	// Store the price update
	ps.storage.Add(*price)

	// Broadcast to all connected clients
	ps.broadcastPrice(*price)
}

// fetchBitcoinPrice fetches the latest Bitcoin price from the CoinDesk API
func (ps *PriceService) fetchBitcoinPrice() (*models.PriceUpdate, error) {
	resp, err := ps.httpClient.Get(ps.apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	var apiResponse models.CoinDeskResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	// Find Bitcoin in the list
	var bitcoinData *models.AssetData
	for _, asset := range apiResponse.Data.List {
		if asset.Symbol == "BTC" {
			bitcoinData = &asset
			break
		}
	}

	if bitcoinData == nil {
		return nil, fmt.Errorf("bitcoin data not found in API response")
	}

	// Convert timestamp from Unix timestamp to time.Time
	timestamp := time.Unix(bitcoinData.PriceUSDLastUpdateTS, 0)

	// Use current time if the API timestamp is too old (more than 1 hour)
	if time.Since(timestamp) > time.Hour {
		timestamp = time.Now()
	}

	priceUpdate := &models.PriceUpdate{
		Timestamp: timestamp,
		Price:     bitcoinData.PriceUSD,
		Symbol:    bitcoinData.Symbol,
		Name:      bitcoinData.Name,
	}

	ps.logger.Infof("Fetched Bitcoin price: $%.2f USD at %s", priceUpdate.Price, priceUpdate.Timestamp.Format(time.RFC3339))

	return priceUpdate, nil
}

// broadcastPrice sends a price update to all connected clients
func (ps *PriceService) broadcastPrice(price models.PriceUpdate) {
	ps.clientsMux.RLock()
	defer ps.clientsMux.RUnlock()

	for clientChan := range ps.clients {
		select {
		case clientChan <- price:
			// Successfully sent
		default:
			// Channel is full or blocked, remove the client
			ps.logger.Warn("Removing blocked client")
			delete(ps.clients, clientChan)
			close(clientChan)
		}
	}
}

// Subscribe adds a new client to receive price updates
func (ps *PriceService) Subscribe() chan models.PriceUpdate {
	clientChan := make(chan models.PriceUpdate, ps.bufferSize)

	ps.clientsMux.Lock()
	ps.clients[clientChan] = true
	ps.clientsMux.Unlock()

	ps.logger.Infof("New client subscribed with buffer size %d. Total clients: %d",
		ps.bufferSize, len(ps.clients))

	return clientChan
}

// Unsubscribe removes a client from receiving price updates
func (ps *PriceService) Unsubscribe(clientChan chan models.PriceUpdate) {
	ps.clientsMux.Lock()
	defer ps.clientsMux.Unlock()

	if _, exists := ps.clients[clientChan]; exists {
		delete(ps.clients, clientChan)
		close(clientChan)
		ps.logger.Infof("Client unsubscribed. Total clients: %d", len(ps.clients))
	}
}

// GetStorage returns the price storage for accessing missed updates
func (ps *PriceService) GetStorage() *storage.PriceStorage {
	return ps.storage
}
