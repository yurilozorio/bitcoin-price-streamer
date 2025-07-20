package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvInt(t *testing.T) {
	// Test with valid environment variable
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	result := GetEnvInt("TEST_INT", 10)
	assert.Equal(t, 42, result)

	// Test with invalid environment variable (non-numeric)
	os.Setenv("TEST_INVALID", "not_a_number")
	defer os.Unsetenv("TEST_INVALID")

	result = GetEnvInt("TEST_INVALID", 10)
	assert.Equal(t, 10, result)

	// Test with zero value (should return default)
	os.Setenv("TEST_ZERO", "0")
	defer os.Unsetenv("TEST_ZERO")

	result = GetEnvInt("TEST_ZERO", 10)
	assert.Equal(t, 10, result)

	// Test with negative value (should return default)
	os.Setenv("TEST_NEGATIVE", "-5")
	defer os.Unsetenv("TEST_NEGATIVE")

	result = GetEnvInt("TEST_NEGATIVE", 10)
	assert.Equal(t, 10, result)

	// Test with empty environment variable
	os.Unsetenv("TEST_EMPTY")
	result = GetEnvInt("TEST_EMPTY", 10)
	assert.Equal(t, 10, result)

	// Test with non-existent environment variable
	result = GetEnvInt("NON_EXISTENT", 10)
	assert.Equal(t, 10, result)
}

func TestGetEnvString(t *testing.T) {
	// Test with valid environment variable
	os.Setenv("TEST_STRING", "hello world")
	defer os.Unsetenv("TEST_STRING")

	result := GetEnvString("TEST_STRING", "default")
	assert.Equal(t, "hello world", result)

	// Test with empty environment variable
	os.Setenv("TEST_EMPTY", "")
	defer os.Unsetenv("TEST_EMPTY")

	result = GetEnvString("TEST_EMPTY", "default")
	assert.Equal(t, "default", result)

	// Test with non-existent environment variable
	result = GetEnvString("NON_EXISTENT", "default")
	assert.Equal(t, "default", result)

	// Test with unset environment variable
	os.Unsetenv("TEST_UNSET")
	result = GetEnvString("TEST_UNSET", "default")
	assert.Equal(t, "default", result)
}

func TestGetEnvIntEdgeCases(t *testing.T) {
	// Test with very large number
	os.Setenv("TEST_LARGE", "999999999")
	defer os.Unsetenv("TEST_LARGE")

	result := GetEnvInt("TEST_LARGE", 10)
	assert.Equal(t, 999999999, result)

	// Test with float string (should return default)
	os.Setenv("TEST_FLOAT", "3.14")
	defer os.Unsetenv("TEST_FLOAT")

	result = GetEnvInt("TEST_FLOAT", 10)
	assert.Equal(t, 10, result)

	// Test with special characters
	os.Setenv("TEST_SPECIAL", "abc123def")
	defer os.Unsetenv("TEST_SPECIAL")

	result = GetEnvInt("TEST_SPECIAL", 10)
	assert.Equal(t, 10, result)
}

func TestGetEnvStringEdgeCases(t *testing.T) {
	// Test with special characters
	os.Setenv("TEST_SPECIAL", "!@#$%^&*()")
	defer os.Unsetenv("TEST_SPECIAL")

	result := GetEnvString("TEST_SPECIAL", "default")
	assert.Equal(t, "!@#$%^&*()", result)

	// Test with unicode characters
	os.Setenv("TEST_UNICODE", "café résumé")
	defer os.Unsetenv("TEST_UNICODE")

	result = GetEnvString("TEST_UNICODE", "default")
	assert.Equal(t, "café résumé", result)

	// Test with spaces and newlines
	os.Setenv("TEST_SPACES", "hello world\nwith newlines")
	defer os.Unsetenv("TEST_SPACES")

	result = GetEnvString("TEST_SPACES", "default")
	assert.Equal(t, "hello world\nwith newlines", result)
}
