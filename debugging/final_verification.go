package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nimda/router-brute/internal/modules/mikrotik/v6"
)

func main() {
	fmt.Println("Final Verification Test")
	fmt.Println("======================")

	// Test with our module to see current behavior
	module := v6.NewMikrotikV6Module()
	module.Initialize("192.168.2.33", "admin", nil)

	// Test various passwords
	passwords := []string{
		"completelywrongpassword123",
		"password",
		"admin",
		"admin1234",
		"", // empty password
	}

	for _, pwd := range passwords {
		success, err := module.Authenticate(nil, pwd)
		if err != nil {
			fmt.Printf("❌ '%s': ERROR - %v\n", pwd, err)
		} else if success {
			fmt.Printf("✅ '%s': SUCCESS\n", pwd)
		} else {
			fmt.Printf("❌ '%s': FAILED (no error)\n", pwd)
		}

		// Small delay between tests
		time.Sleep(200 * time.Millisecond)
	}

	module.Close()

	fmt.Println("\n📋 Analysis:")
	fmt.Println("If ALL passwords succeed, the router has a security issue.")
	fmt.Println("If ONLY 'admin1234' succeeds, our implementation is correct.")
	fmt.Println("If NONE succeed, there might be a connection/authentication issue.")
}
