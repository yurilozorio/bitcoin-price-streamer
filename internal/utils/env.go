package utils

import (
	"os"
	"strconv"
)

// GetEnvInt retrieves an environment variable as an integer
// Returns defaultValue if the environment variable is not set, empty, or invalid
func GetEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil && intValue > 0 {
			return intValue
		}
	}
	return defaultValue
}

// GetEnvString retrieves an environment variable as a string
// Returns defaultValue if the environment variable is not set
func GetEnvString(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
