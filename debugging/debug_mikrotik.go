package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/nimda/router-brute/internal/modules/mikrotik/v6"
)

func main() {
	fmt.Println("Mikrotik RouterOS v6 Debug Test")
	fmt.Println("==============================")

	// Create module with debug wrapper
	baseModule := v6.NewMikrotikV6Module()
	module := &v6.DebugMikrotikV6{baseModule}
	module.Initialize("192.168.2.33", "admin", map[string]interface{}{
		"port":    8728,
		"timeout": 5 * time.Second,
	})

	fmt.Printf("Target: %s\n", module.GetTarget())
	fmt.Printf("Username: %s\n", module.GetUsername())
	fmt.Printf("Protocol: %s\n", module.GetProtocolName())
	fmt.Printf("Port: %d\n", 8728)
	fmt.Printf("Timeout: %v\n", 5*time.Second)

	// Test basic TCP connection
	fmt.Println("\nTesting TCP connection...")

	address := fmt.Sprintf("%s:%d", module.GetTarget(), 8728)
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)

	if err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		fmt.Println("\nPossible issues:")
		fmt.Println("1. API service not enabled on Mikrotik")
		fmt.Println("2. Firewall blocking port 8728")
		fmt.Println("3. Wrong IP address")
		fmt.Println("4. Network connectivity issues")
		return
	}

	defer conn.Close()
	fmt.Printf("✅ TCP connection successful to %s\n", address)

	// Test protocol handshake
	fmt.Println("\nTesting protocol handshake...")

	// Try to send a simple command
	command := "/login"
	loginData := map[string]string{
		"name":     "admin",
		"password": "admin1234",
	}

	// Build sentence
	sentence := []string{command}
	for k, v := range loginData {
		sentence = append(sentence, k+"="+v)
	}

	// Encode sentence
	encoded, err := module.EncodeSentence(sentence)
	if err != nil {
		fmt.Printf("❌ Encoding failed: %v\n", err)
		return
	}

	fmt.Printf("Encoded command: %v\n", sentence)
	fmt.Printf("Encoded bytes: %d\n", len(encoded))

	// Send command
	_, err = conn.Write(encoded)
	if err != nil {
		fmt.Printf("❌ Write failed: %v\n", err)
		return
	}

	fmt.Println("✅ Command sent successfully")

	// Read response
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)

	if err != nil {
		fmt.Printf("❌ Read failed: %v\n", err)
		return
	}

	response := buf[:n]
	fmt.Printf("✅ Received response (%d bytes): %v\n", n, response)

	// Try to decode response
	words, err := module.DecodeWords(response)
	if err != nil {
		fmt.Printf("❌ Decoding failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Decoded words: %v\n", words)

	// Use the actual module's authentication logic
	ctx := context.Background()
	success, err := baseModule.Authenticate(ctx, "admin1234")
	if err != nil {
		fmt.Printf("❌ Authentication error: %v\n", err)
	} else if success {
		fmt.Println("🎉 Authentication successful!")
	} else {
		fmt.Println("❌ Authentication failed (wrong credentials)")
	}

	fmt.Println("\nDebug test completed")
}
