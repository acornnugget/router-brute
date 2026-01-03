package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/nimda/router-brute/internal/modules/mikrotik/v6"
)

func main() {
	fmt.Println("Detailed Mikrotik v6 Protocol Debug")
	fmt.Println("===================================")

	// Test multiple passwords to see the pattern
	passwords := []string{"password", "123456", "admin", "admin1234", "wrongpass"}

	for i, password := range passwords {
		fmt.Printf("\n=== Test %d: Password '%s' ===\n", i+1, password)

		// Create fresh module for each test
		module := v6.NewMikrotikV6Module()
		module.Initialize("192.168.2.33", "admin", map[string]interface{}{
			"port":    8728,
			"timeout": 5 * time.Second,
		})

		// Test authentication
		ctx := context.Background()
		success, err := module.Authenticate(ctx, password)

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		if success {
			fmt.Printf("✅ SUCCESS with password: %s\n", password)
		} else {
			fmt.Printf("❌ FAILED with password: %s\n", password)
		}

		// Clean up
		module.Close()

		// Small delay between tests
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("\n=== Manual Protocol Test ===")
	testManualProtocol()
}

func testManualProtocol() {
	// Test the raw protocol manually
	conn, err := net.DialTimeout("tcp", "192.168.2.33:8728", 5*time.Second)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		return
	}
	defer conn.Close()

	// Test with wrong password first
	fmt.Println("Testing with wrong password 'wrongpass'...")
	sentence := []string{"/login", "name=admin", "password=wrongpass"}

	// Encode and send
	module := v6.NewMikrotikV6Module()
	debugModule := &v6.DebugMikrotikV6{module}

	encoded, _ := debugModule.EncodeSentence(sentence)
	conn.Write(encoded)

	// Read response
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("Read failed: %v", err)
		return
	}

	response := buf[:n]
	words, _ := debugModule.DecodeWords(response)
	fmt.Printf("Response for wrong password: %v\n", words)

	// Close and reconnect for correct password
	conn.Close()

	conn2, err := net.DialTimeout("tcp", "192.168.2.33:8728", 5*time.Second)
	if err != nil {
		log.Printf("Second connection failed: %v", err)
		return
	}
	defer conn2.Close()

	// Test with correct password
	fmt.Println("Testing with correct password 'admin1234'...")
	sentence2 := []string{"/login", "name=admin", "password=admin1234"}
	encoded2, _ := debugModule.EncodeSentence(sentence2)
	conn2.Write(encoded2)

	// Read response
	buf2 := make([]byte, 1024)
	conn2.SetReadDeadline(time.Now().Add(3 * time.Second))
	n2, err := conn2.Read(buf2)
	if err != nil {
		log.Printf("Second read failed: %v", err)
		return
	}

	response2 := buf2[:n2]
	words2, _ := debugModule.DecodeWords(response2)
	fmt.Printf("Response for correct password: %v\n", words2)

	// Analyze responses
	fmt.Println("\n=== Analysis ===")
	fmt.Printf("Wrong password response contains '!trap' or '!fatal': %t\n",
		containsError(words))
	fmt.Printf("Correct password response contains '=ret=': %t\n",
		containsSuccess(words2))
}

func containsError(words []string) bool {
	for _, word := range words {
		if word == "!trap" || word == "!fatal" {
			return true
		}
	}
	return false
}

func containsSuccess(words []string) bool {
	for _, word := range words {
		if word == "=ret=" || len(word) > 5 && word[:5] == "=ret=" {
			return true
		}
	}
	return false
}
