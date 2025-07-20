package storage

import (
	"context"
	"testing"
	"time"

	"bitcoin-price-streamer/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewPriceStorage(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()

	storage := NewPriceStorage(ctx, 100, logger)

	assert.NotNil(t, storage)
	assert.Equal(t, 100, storage.capacity)
	assert.Equal(t, 0, storage.size)
	assert.Equal(t, 0, storage.head)
	assert.Equal(t, 0, storage.tail)
}

func TestAddAndGetAllUpdates(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()
	storage := NewPriceStorage(ctx, 3, logger)

	// Add updates
	update1 := models.PriceUpdate{
		Timestamp: time.Now().Add(-2 * time.Second),
		Price:     100.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}
	update2 := models.PriceUpdate{
		Timestamp: time.Now().Add(-1 * time.Second),
		Price:     101.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}
	update3 := models.PriceUpdate{
		Timestamp: time.Now(),
		Price:     102.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}

	storage.Add(update1)
	storage.Add(update2)
	storage.Add(update3)

	updates := storage.GetAllUpdates()
	assert.Len(t, updates, 3)
	assert.Equal(t, update1.Price, updates[0].Price)
	assert.Equal(t, update2.Price, updates[1].Price)
	assert.Equal(t, update3.Price, updates[2].Price)
}

func TestRingBufferWrapping(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()
	storage := NewPriceStorage(ctx, 2, logger)

	// Add 3 updates to a buffer of size 2
	update1 := models.PriceUpdate{
		Timestamp: time.Now().Add(-2 * time.Second),
		Price:     100.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}
	update2 := models.PriceUpdate{
		Timestamp: time.Now().Add(-1 * time.Second),
		Price:     101.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}
	update3 := models.PriceUpdate{
		Timestamp: time.Now(),
		Price:     102.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}

	storage.Add(update1)
	storage.Add(update2)
	storage.Add(update3) // This should overwrite update1

	updates := storage.GetAllUpdates()
	assert.Len(t, updates, 2)
	assert.Equal(t, update2.Price, updates[0].Price) // Oldest remaining
	assert.Equal(t, update3.Price, updates[1].Price) // Newest

	// after wrap around, the head and tail should be 1
	assert.Equal(t, 2, storage.size)
	assert.Equal(t, 1, storage.head)
	assert.Equal(t, 1, storage.tail)
}

func TestGetUpdatesSince(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()
	storage := NewPriceStorage(ctx, 5, logger)

	baseTime := time.Now()

	update1 := models.PriceUpdate{
		Timestamp: baseTime.Add(-10 * time.Second),
		Price:     100.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}
	update2 := models.PriceUpdate{
		Timestamp: baseTime.Add(-5 * time.Second),
		Price:     101.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}
	update3 := models.PriceUpdate{
		Timestamp: baseTime,
		Price:     102.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}

	storage.Add(update1)
	storage.Add(update2)
	storage.Add(update3)

	// Get updates since 7 seconds ago (should get update2 and update3)
	since := baseTime.Add(-7 * time.Second)
	updates := storage.GetUpdatesSince(since)

	assert.Len(t, updates, 2)
	assert.Equal(t, update2.Price, updates[0].Price)
	assert.Equal(t, update3.Price, updates[1].Price)
}

func TestGetLatest(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()
	storage := NewPriceStorage(ctx, 3, logger)

	// Empty storage
	latest, exists := storage.GetLatest()
	assert.False(t, exists)
	assert.Equal(t, models.PriceUpdate{}, latest)

	// Add updates
	update1 := models.PriceUpdate{
		Timestamp: time.Now().Add(-2 * time.Second),
		Price:     100.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}
	update2 := models.PriceUpdate{
		Timestamp: time.Now().Add(-1 * time.Second),
		Price:     101.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}

	storage.Add(update1)
	storage.Add(update2)

	latest, exists = storage.GetLatest()
	assert.True(t, exists)
	assert.Equal(t, update2.Price, latest.Price)
}

func TestGetLatestWithWrapping(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()
	storage := NewPriceStorage(ctx, 2, logger)

	update1 := models.PriceUpdate{
		Timestamp: time.Now().Add(-2 * time.Second),
		Price:     100.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}
	update2 := models.PriceUpdate{
		Timestamp: time.Now().Add(-1 * time.Second),
		Price:     101.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}
	update3 := models.PriceUpdate{
		Timestamp: time.Now(),
		Price:     102.0,
		Symbol:    "BTC",
		Name:      "Bitcoin",
	}

	storage.Add(update1)
	storage.Add(update2)
	storage.Add(update3) // Overwrites update1

	latest, exists := storage.GetLatest()
	assert.True(t, exists)
	assert.Equal(t, update3.Price, latest.Price)
}

func TestConcurrentAccess(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()
	storage := NewPriceStorage(ctx, 100, logger)

	// Test concurrent adds
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			update := models.PriceUpdate{
				Timestamp: time.Now(),
				Price:     float64(id),
				Symbol:    "BTC",
				Name:      "Bitcoin",
			}
			storage.Add(update)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	updates := storage.GetAllUpdates()
	assert.Len(t, updates, 10)
}

func TestStorageCapacity(t *testing.T) {
	logger := logrus.New()
	ctx := context.Background()
	storage := NewPriceStorage(ctx, 5, logger)

	// Add more items than capacity
	for i := 0; i < 10; i++ {
		update := models.PriceUpdate{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Price:     float64(i),
			Symbol:    "BTC",
			Name:      "Bitcoin",
		}
		storage.Add(update)
	}

	// Should only have the last 5 items
	updates := storage.GetAllUpdates()
	assert.Len(t, updates, 5)

	// Check that we have the most recent items
	expectedPrices := []float64{5, 6, 7, 8, 9}
	for i, update := range updates {
		assert.Equal(t, expectedPrices[i], update.Price)
	}
}
