package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/nimda/router-brute/internal/modules/mikrotik/v6"
)

func main() {
	fmt.Println("Deep Mikrotik Protocol Analysis")
	fmt.Println("===============================")

	// Test the exact protocol step by step
	testProtocolStepByStep()

	// Test our module vs manual implementation
	testModuleVsManual()
}

func testProtocolStepByStep() {
	fmt.Println("\n=== Step-by-Step Protocol Test ===")

	// Connect to router
	conn, err := net.DialTimeout("tcp", "192.168.2.33:8728", 5*time.Second)
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer conn.Close()

	// Create debug module
	module := v6.NewMikrotikV6Module()
	debugModule := &v6.DebugMikrotikV6{module}

	// Test Case 1: Wrong password
	fmt.Println("\n--- Test 1: Wrong Password 'password' ---")
	sentence1 := []string{"/login", "name=admin", "password=password"}
	encoded1, _ := debugModule.EncodeSentence(sentence1)

	fmt.Printf("Sentence: %v\n", sentence1)
	fmt.Printf("Encoded: %s\n", hex.EncodeToString(encoded1))

	// Send to router
	_, err = conn.Write(encoded1)
	if err != nil {
		log.Printf("Write failed: %v", err)
		return
	}

	// Read response
	buf1 := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n1, err := conn.Read(buf1)
	if err != nil {
		log.Printf("Read failed: %v", err)
		return
	}

	response1 := buf1[:n1]
	words1, _ := debugModule.DecodeWords(response1)

	fmt.Printf("Raw response: %s\n", hex.EncodeToString(response1))
	fmt.Printf("Decoded words: %v\n", words1)
	fmt.Printf("Contains error: %t\n", containsError(words1))
	fmt.Printf("Contains success: %t\n", containsSuccess(words1))

	// Test Case 2: Correct password
	fmt.Println("\n--- Test 2: Correct Password 'admin1234' ---")

	// Reconnect for clean session
	conn.Close()
	conn2, err := net.DialTimeout("tcp", "192.168.2.33:8728", 5*time.Second)
	if err != nil {
		log.Fatalf("Reconnection failed: %v", err)
	}

	sentence2 := []string{"/login", "name=admin", "password=admin1234"}
	encoded2, _ := debugModule.EncodeSentence(sentence2)

	fmt.Printf("Sentence: %v\n", sentence2)
	fmt.Printf("Encoded: %s\n", hex.EncodeToString(encoded2))

	_, err = conn2.Write(encoded2)
	if err != nil {
		log.Printf("Write failed: %v", err)
		conn2.Close()
		return
	}

	buf2 := make([]byte, 1024)
	conn2.SetReadDeadline(time.Now().Add(3 * time.Second))
	n2, err := conn2.Read(buf2)
	if err != nil {
		log.Printf("Read failed: %v", err)
		conn2.Close()
		return
	}

	response2 := buf2[:n2]
	words2, _ := debugModule.DecodeWords(response2)

	fmt.Printf("Raw response: %s\n", hex.EncodeToString(response2))
	fmt.Printf("Decoded words: %v\n", words2)
	fmt.Printf("Contains error: %t\n", containsError(words2))
	fmt.Printf("Contains success: %t\n", containsSuccess(words2))

	conn2.Close()

	// Analysis
	fmt.Println("\n=== Analysis ===")
	fmt.Printf("Wrong password should contain error: %t\n", containsError(words1))
	fmt.Printf("Correct password should contain success: %t\n", containsSuccess(words2))

	if !containsError(words1) && containsSuccess(words2) {
		fmt.Println("❌ BOTH responses show success - this indicates a protocol issue")
	} else if containsError(words1) && containsSuccess(words2) {
		fmt.Println("✅ Protocol working correctly - wrong fails, correct succeeds")
	} else if !containsError(words1) && !containsSuccess(words2) {
		fmt.Println("❓ Unexpected response pattern")
	} else {
		fmt.Println("❓ Mixed response pattern")
	}
}

func testModuleVsManual() {
	fmt.Println("\n=== Module vs Manual Implementation ===")

	// Test with our module
	fmt.Println("\n--- Using Our Module ---")
	module := v6.NewMikrotikV6Module()
	module.Initialize("192.168.2.33", "admin", nil)

	// Test wrong password
	ctx := context.Background()
	success1, err1 := module.Authenticate(ctx, "password")
	module.Close()

	// Test correct password
	module2 := v6.NewMikrotikV6Module()
	module2.Initialize("192.168.2.33", "admin", nil)
	success2, err2 := module2.Authenticate(ctx, "admin1234")
	module2.Close()

	fmt.Printf("Wrong password result: success=%t, error=%v\n", success1, err1)
	fmt.Printf("Correct password result: success=%t, error=%v\n", success2, err2)

	// Expected behavior
	if !success1 && success2 && err1 == nil && err2 == nil {
		fmt.Println("✅ Module working correctly")
	} else if success1 && success2 {
		fmt.Println("❌ Module accepting all passwords (protocol issue)")
	} else {
		fmt.Println("❓ Unexpected module behavior")
	}
}

func containsError(words []string) bool {
	for _, word := range words {
		if word == "!trap" || word == "!fatal" || word == "!done" {
			return true
		}
		if strings.HasPrefix(word, "=message=") {
			return true
		}
	}
	return false
}

func containsSuccess(words []string) bool {
	for _, word := range words {
		if word == "=ret=" {
			return true
		}
		if len(word) > 5 && word[:5] == "=ret=" {
			return true
		}
	}
	return false
}
