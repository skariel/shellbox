package test

import (
	"os"
	"strconv"
	"time"
)

// Config holds configuration for test execution
type Config struct {
	// Resource configuration
	ResourceGroupPrefix string
	Location            string
	CleanupTimeout      time.Duration

	// Execution configuration
	ParallelLimit int
	TestTimeout   time.Duration
	IsCI          bool

	// Azure configuration
	UseAzureCLI bool
}

// DefaultConfig returns the default test configuration
func DefaultConfig() *Config {
	return &Config{
		ResourceGroupPrefix: "test",
		Location:            "westus2",
		CleanupTimeout:      10 * time.Minute,
		ParallelLimit:       4,
		TestTimeout:         60 * time.Minute,
		IsCI:                false,
		UseAzureCLI:         true,
	}
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	config := DefaultConfig()

	// Resource configuration
	if prefix := os.Getenv("TEST_RESOURCE_GROUP_PREFIX"); prefix != "" {
		config.ResourceGroupPrefix = prefix
	}
	if location := os.Getenv("TEST_LOCATION"); location != "" {
		config.Location = location
	}
	if timeout := os.Getenv("TEST_CLEANUP_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			config.CleanupTimeout = d
		}
	}

	// Execution configuration
	if limit := os.Getenv("TEST_PARALLEL_LIMIT"); limit != "" {
		if i, err := strconv.Atoi(limit); err == nil {
			config.ParallelLimit = i
		}
	}
	if timeout := os.Getenv("TEST_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			config.TestTimeout = d
		}
	}

	// CI detection
	config.IsCI = parseBool(os.Getenv("CI"), false) ||
		parseBool(os.Getenv("GITHUB_ACTIONS"), false) ||
		parseBool(os.Getenv("AZURE_DEVOPS"), false)

	// Azure configuration
	config.UseAzureCLI = os.Getenv("AZURE_CLIENT_ID") == ""

	// CI-specific adjustments
	if config.IsCI {
		config.ParallelLimit = 2 // More conservative in CI
		config.TestTimeout = 90 * time.Minute
	}

	return config
}

// parseBool parses a string to bool with default fallback
func parseBool(s string, defaultValue bool) bool {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.ParseBool(s)
	if err != nil {
		return defaultValue
	}
	return val
}
