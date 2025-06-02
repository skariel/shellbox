package main

import (
	"fmt"
	"log"

	"shellbox/internal/infra"
)

func main() {
	// Test suffix from the failing test
	testSuffix := "test-integration-68396"

	fmt.Println("=== Hash Debug Tool ===")
	fmt.Printf("Test suffix: %s\n\n", testSuffix)

	// Generate the formatted config string
	configString := infra.FormatConfig(testSuffix)
	fmt.Println("=== Formatted Config String ===")
	fmt.Println(configString)
	fmt.Println()

	// Generate the hash
	hash, err := infra.GenerateConfigHash(testSuffix)
	if err != nil {
		log.Fatalf("Failed to generate hash: %v", err)
	}

	fmt.Println("=== Generated Hash ===")
	fmt.Printf("Hash: %s\n", hash)
	fmt.Printf("Hash length: %d characters\n", len(hash))

	// Also test with a few other suffixes for comparison
	fmt.Println("\n=== Additional Test Cases ===")
	testCases := []string{
		"test",
		"integration",
		"test-integration",
		"test-integration-12345",
	}

	for _, suffix := range testCases {
		hash, err := infra.GenerateConfigHash(suffix)
		if err != nil {
			fmt.Printf("Error generating hash for '%s': %v\n", suffix, err)
			continue
		}
		fmt.Printf("Suffix: %-25s Hash: %s\n", suffix, hash)
	}
}
