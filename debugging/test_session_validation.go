package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/nimda/router-brute/internal/modules/mikrotik/v6"
)

func main() {
	fmt.Println("Mikrotik Session Validation Test")
	fmt.Println("================================")

	// Test the theory: after login, send a simple command to validate session
	testSessionWithCommand("password", "WRONG")
	time.Sleep(500 * time.Millisecond)
	testSessionWithCommand("admin1234", "CORRECT")
}

func testSessionWithCommand(password, label string) {
	fmt.Printf("\n=== Testing %s password: '%s' ===\n", label, password)

	// Step 1: Login
	conn, err := net.DialTimeout("tcp", "192.168.2.33:8728", 5*time.Second)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		return
	}

	module := v6.NewMikrotikV6Module()
	debugModule := &v6.DebugMikrotikV6{module}

	// Send login
	sentence := []string{"/login", "name=admin", "password=" + password}
	encoded, _ := debugModule.EncodeSentence(sentence)

	fmt.Printf("Login sentence: %v\n", sentence)
	fmt.Printf("Encoded: %s\n", hex.EncodeToString(encoded))

	_, err = conn.Write(encoded)
	if err != nil {
		log.Printf("Write failed: %v", err)
		conn.Close()
		return
	}

	// Read login response
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("Read failed: %v", err)
		conn.Close()
		return
	}

	response := buf[:n]
	words, _ := debugModule.DecodeWords(response)
	fmt.Printf("Login response: %v\n", words)

	// Step 2: Try to send a simple command to validate session
	// Only send this if we got a session ID (indicated by =ret=)
	hasSession := false
	for _, word := range words {
		if len(word) > 5 && word[:5] == "=ret=" {
			hasSession = true
			break
		}
	}

	if hasSession {
		fmt.Println("Session established, testing with simple command...")

		// Send a simple command that should work if session is valid
		simpleCmd := []string{"/system/identity/print"}
		encodedCmd, _ := debugModule.EncodeSentence(simpleCmd)

		fmt.Printf("Command: %v\n", simpleCmd)
		fmt.Printf("Encoded: %s\n", hex.EncodeToString(encodedCmd))

		_, err = conn.Write(encodedCmd)
		if err != nil {
			log.Printf("Command write failed: %v", err)
			conn.Close()
			return
		}

		// Read command response
		buf2 := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n2, err := conn.Read(buf2)
		if err != nil {
			log.Printf("Command read failed: %v", err)
			conn.Close()
			return
		}

		response2 := buf2[:n2]
		words2, _ := debugModule.DecodeWords(response2)
		fmt.Printf("Command response: %v\n", words2)

		// Analyze command response
		if containsError(words2) {
			fmt.Printf("❌ %s: Session rejected by router (command failed)\n", label)
		} else {
			fmt.Printf("✅ %s: Session accepted by router (command succeeded)\n", label)
		}
	} else {
		fmt.Printf("❌ %s: No session established\n", label)
	}

	conn.Close()
}

func containsError(words []string) bool {
	for _, word := range words {
		if word == "!trap" || word == "!fatal" {
			return true
		}
	}
	return false
}
