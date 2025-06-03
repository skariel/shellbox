package test

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Category represents a category of tests with different execution characteristics
type Category string

const (
	CategoryUnit        Category = "unit"        // < 30s, pure Go logic
	CategoryIntegration Category = "integration" // < 10m, infrastructure
	CategoryE2E         Category = "e2e"         // < 45m, end-to-end scenarios
)

// Config holds configuration for test execution
type Config struct {
	// Category selection
	Categories []Category

	// Skip flags for expensive operations
	SkipE2ETests       bool

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
		Categories:          []Category{CategoryUnit},
		SkipE2ETests:        false,
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

	// Load categories from TEST_CATEGORIES
	if categories := os.Getenv("TEST_CATEGORIES"); categories != "" {
		config.Categories = parseCategories(categories)
	}

	// Skip flags
	config.SkipE2ETests = parseBool(os.Getenv("SKIP_E2E_TESTS"), false)

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

// ShouldRunCategory returns true if the given category should be executed
func (c *Config) ShouldRunCategory(category Category) bool {
	// Check skip flags first
	switch category {
	case CategoryE2E:
		if c.SkipE2ETests {
			return false
		}
	}

	// Check if category is in the allowed list
	for _, allowed := range c.Categories {
		if allowed == category {
			return true
		}
	}

	return false
}

// AllCategories returns all available test categories
func AllCategories() []Category {
	return []Category{
		CategoryUnit,
		CategoryIntegration,
		CategoryE2E,
	}
}

// FastCategories returns categories that run quickly (< 30s)
func FastCategories() []Category {
	return []Category{
		CategoryUnit,
	}
}

// SlowCategories returns categories that take significant time (> 10 minutes)
func SlowCategories() []Category {
	return []Category{
		CategoryIntegration,
		CategoryE2E,
	}
}

// parseCategories parses a comma-separated list of categories
func parseCategories(categoriesStr string) []Category {
	if categoriesStr == "all" {
		return AllCategories()
	}
	if categoriesStr == "fast" {
		return FastCategories()
	}
	if categoriesStr == "slow" {
		return SlowCategories()
	}

	var categories []Category
	for _, cat := range strings.Split(categoriesStr, ",") {
		cat = strings.TrimSpace(cat)
		if cat != "" {
			categories = append(categories, Category(cat))
		}
	}
	return categories
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

// GetEstimatedDuration returns the estimated duration for a category
func GetEstimatedDuration(category Category) time.Duration {
	switch category {
	case CategoryUnit:
		return 30 * time.Second
	case CategoryIntegration:
		return 10 * time.Minute
	case CategoryE2E:
		return 45 * time.Minute
	default:
		return 5 * time.Minute
	}
}
