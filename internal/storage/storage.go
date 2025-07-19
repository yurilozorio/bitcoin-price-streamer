package storage

import (
	"context"
	"sync"
	"time"

	"bitcoin-price-streamer/internal/models"

	"github.com/sirupsen/logrus"
)

// PriceStorage manages price updates with ring buffer
type PriceStorage struct {
	updates  []models.PriceUpdate
	capacity int
	head     int
	tail     int
	size     int
	mutex    sync.RWMutex
	logger   *logrus.Logger
}

func NewPriceStorage(ctx context.Context, capacity int, logger *logrus.Logger) *PriceStorage {
	return &PriceStorage{
		updates:  make([]models.PriceUpdate, capacity),
		capacity: capacity,
		logger:   logger,
	}
}

// Add adds a new price update to the storage
func (ps *PriceStorage) Add(update models.PriceUpdate) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	ps.updates[ps.head] = update
	ps.head = (ps.head + 1) % ps.capacity

	if ps.size < ps.capacity {
		// if size is less than capacity, only head moves
		ps.size++
	} else {
		// if buffer is full (size = capacity), also move tail
		ps.tail = (ps.tail + 1) % ps.capacity
	}

	ps.logger.Debugf("Added price update: $%.2f at %s (storage size: %d/%d)",
		update.Price, update.Timestamp.Format(time.RFC3339), ps.size, ps.capacity)
}

// GetUpdatesSince returns all updates since the given timestamp
func (ps *PriceStorage) GetUpdatesSince(since time.Time) []models.PriceUpdate {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	var updates []models.PriceUpdate

	for i := 0; i < ps.size; i++ {
		idx := (ps.tail + i) % ps.capacity
		update := ps.updates[idx]

		// Only check if update is after the requested time
		if update.Timestamp.After(since) {
			updates = append(updates, update)
		}
	}

	ps.logger.Debugf("Retrieved %d updates since %s", len(updates), since.Format(time.RFC3339))
	return updates
}

// GetAllUpdates returns all stored updates
func (ps *PriceStorage) GetAllUpdates() []models.PriceUpdate {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	updates := make([]models.PriceUpdate, ps.size)
	for i := 0; i < ps.size; i++ {
		idx := (ps.tail + i) % ps.capacity
		updates[i] = ps.updates[idx]
	}

	ps.logger.Debugf("Retrieved all %d updates", len(updates))
	return updates
}

// GetLatest returns the most recent price update
func (ps *PriceStorage) GetLatest() (models.PriceUpdate, bool) {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	if ps.size == 0 {
		ps.logger.Debug("No price updates available")
		return models.PriceUpdate{}, false
	}

	latest := ps.updates[(ps.head-1+ps.capacity)%ps.capacity]

	ps.logger.Debugf("Retrieved latest price: $%.2f at %s",
		latest.Price, latest.Timestamp.Format(time.RFC3339))
	return latest, true
}
