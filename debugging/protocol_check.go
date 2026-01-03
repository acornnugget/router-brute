package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	fmt.Println("Simple Mikrotik Protocol Test")
	fmt.Println("=============================")

	// Test wrong password
	testPassword("password", "WRONG")
	time.Sleep(500 * time.Millisecond)

	// Test correct password
	testPassword("admin1234", "CORRECT")
}

func testPassword(password, label string) {
	fmt.Printf("\n=== %s Password: '%s' ===\n", label, password)

	// Connect to router
	conn, err := net.DialTimeout("tcp", "192.168.2.33:8728", 5*time.Second)
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	// Send login command using binary protocol
	command := buildLoginCommand("admin", password)
	_, err = conn.Write(command)
	if err != nil {
		fmt.Printf("Write failed: %v\n", err)
		return
	}

	// Read response
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("Read failed: %v\n", err)
		return
	}

	response := buf[:n]
	fmt.Printf("Raw response (%d bytes): %v\n", n, response)
	fmt.Printf("Response as string: %s\n", string(response))

	// Parse the response
	words := parseResponse(response)
	fmt.Printf("Parsed words: %v\n", words)

	// Check for error indicators
	hasError := false
	hasSuccess := false

	for _, word := range words {
		if word == "!trap" || word == "!fatal" {
			hasError = true
		}
		if len(word) >= 5 && word[:5] == "=ret=" {
			hasSuccess = true
		}
	}

	fmt.Printf("Contains error indicators: %t\n", hasError)
	fmt.Printf("Contains success indicators: %t\n", hasSuccess)

	if hasError {
		fmt.Printf("❌ %s: Router returned error\n", label)
	} else if hasSuccess {
		fmt.Printf("✅ %s: Router returned success (has =ret=)\n", label)
	} else {
		// !done alone means success for Mikrotik
		fmt.Printf("✅ %s: Router returned success (!done)\n", label)
	}
}

func buildLoginCommand(username, password string) []byte {
	// Mikrotik binary protocol: length-prefixed words
	var buf []byte

	// Add /login command
	buf = appendLengthPrefixed(buf, "/login")

	// Add username
	buf = appendLengthPrefixed(buf, "=name="+username)

	// Add password
	buf = appendLengthPrefixed(buf, "=password="+password)

	// Add null terminator
	buf = append(buf, 0x00)

	return buf
}

func appendLengthPrefixed(buf []byte, word string) []byte {
	wordBytes := []byte(word)
	if len(wordBytes) > 255 {
		// Truncate if too long (shouldn't happen)
		wordBytes = wordBytes[:255]
	}
	buf = append(buf, byte(len(wordBytes)))
	buf = append(buf, wordBytes...)
	return buf
}

func parseResponse(data []byte) []string {
	var words []string
	i := 0

	for i < len(data) {
		if data[i] == 0x00 {
			break // End of sentence
		}

		length := int(data[i])
		i++

		if i+length > len(data) {
			break
		}

		word := string(data[i : i+length])
		words = append(words, word)
		i += length
	}

	return words
}
