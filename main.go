package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bitcoin-price-streamer/internal/handlers"
	"bitcoin-price-streamer/internal/service"
	"bitcoin-price-streamer/internal/storage"
	"bitcoin-price-streamer/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize storage for missed updates with configurable capacity
	storageCapacity := utils.GetEnvInt("STORAGE_CAPACITY", 1000)
	storage := storage.NewPriceStorage(ctx, storageCapacity, logger)

	// Initialize price service
	priceService := service.NewPriceService(storage, logger)

	// Start price polling in background
	go priceService.StartPolling(ctx)

	// Initialize handlers
	handlers := handlers.NewHandlers(priceService, logger)

	// Setup Gin router
	router := gin.Default()

	// Setup routes
	handlers.SetupRoutes(router)

	// Get port from environment or use default
	port := utils.GetEnvString("PORT", "8080")

	// Create HTTP server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server in background
	go func() {
		logger.Infof("Starting server on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited gracefully")
}
