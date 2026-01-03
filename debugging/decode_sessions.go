package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	fmt.Println("Session ID Analysis")
	fmt.Println("===================")

	// Test multiple times to collect session IDs
	fmt.Println("\n=== Wrong Password Tests ===")
	for i := 0; i < 3; i++ {
		testPassword("password", "WRONG")
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("\n=== Correct Password Tests ===")
	for i := 0; i < 3; i++ {
		testPassword("admin1234", "CORRECT")
		time.Sleep(200 * time.Millisecond)
	}
}

func testPassword(password, label string) {
	conn, err := net.DialTimeout("tcp", "192.168.2.33:8728", 5*time.Second)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		return
	}
	defer conn.Close()

	// Send login command
	command := buildLoginCommand("admin", password)
	_, err = conn.Write(command)
	if err != nil {
		log.Printf("Write failed: %v", err)
		return
	}

	// Read response
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("Read failed: %v", err)
		return
	}

	response := buf[:n]
	words := parseResponse(response)

	// Extract and analyze session ID if present
	for _, word := range words {
		if len(word) > 5 && word[:5] == "=ret=" {
			sessionID := word[5:]
			fmt.Printf("%s: Session ID: %s (hex: %s, len: %d)\n",
				label, sessionID, hex.EncodeToString([]byte(sessionID)), len(sessionID))

			// Analyze session ID characteristics
			analyzeSessionID(sessionID, label)
		}
	}

	fmt.Printf("%s: Full response: %v\n", label, words)
}

func analyzeSessionID(sessionID, label string) {
	// Check for patterns in session IDs
	if len(sessionID) > 0 {
		// Check first character
		firstChar := sessionID[0]
		// Check if it's hex-like
		isHex := true
		for _, c := range sessionID {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				isHex = false
				break
			}
		}
		fmt.Printf("%s: First char: %c, Is hex: %t\n", label, firstChar, isHex)
	}
}

// Copy functions from protocol_check.go
func buildLoginCommand(username, password string) []byte {
	var buf []byte
	buf = appendLengthPrefixed(buf, "/login")
	buf = appendLengthPrefixed(buf, "=name="+username)
	buf = appendLengthPrefixed(buf, "=password="+password)
	buf = append(buf, 0x00)
	return buf
}

func appendLengthPrefixed(buf []byte, word string) []byte {
	wordBytes := []byte(word)
	if len(wordBytes) > 255 {
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
			break
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
