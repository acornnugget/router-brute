package main

import (
	"fmt"
	"time"

	"github.com/nimda/router-brute/internal/core"
	"github.com/nimda/router-brute/internal/modules/mikrotik/v6"
)

func main() {
	fmt.Println("Simple Mikrotik v6 Test")
	fmt.Println("======================")

	// Create module
	module := v6.NewMikrotikV6Module()
	module.Initialize("192.168.2.33", "admin", map[string]interface{}{
		"port":    8728,
		"timeout": 10 * time.Second,
	})

	// Create engine with fast settings
	engine := core.NewEngine(1, 100*time.Millisecond)
	engine.SetModule(module)

	// Load passwords (include the correct one)
	passwords := []string{"wrong1", "wrong2", "admin1234", "wrong3"}
	engine.LoadPasswords(passwords)

	fmt.Printf("Testing %d passwords...\n", len(passwords))

	// Start the engine
	if err := engine.Start(); err != nil {
		fmt.Printf("Failed to start engine: %v\n", err)
		return
	}

	// Monitor results
	successCount := 0
	for result := range engine.Results() {
		fmt.Printf("Attempt: %s - ", result.Password)

		if result.Success {
			fmt.Println("✅ SUCCESS!")
			successCount++
			engine.Stop()
			break
		} else {
			fmt.Println("❌ FAILED")
		}
	}

	fmt.Printf("\nTest completed. Successes: %d\n", successCount)

	// Give it a moment to clean up
	time.Sleep(500 * time.Millisecond)

	if successCount > 0 {
		fmt.Println("🎉 Mikrotik v6 implementation working!")
	} else {
		fmt.Println("❌ No successful authentications")
	}
}
