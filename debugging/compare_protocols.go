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
	fmt.Println("Side-by-Side Comparison Test")
	fmt.Println("===========================")

	// Test 1: Using our simple protocol (like protocol_check.go)
	fmt.Println("\n=== Test 1: Simple Protocol (Direct TCP) ===")
	testSimpleProtocol("password", "WRONG")
	time.Sleep(500 * time.Millisecond)
	testSimpleProtocol("admin1234", "CORRECT")

	// Test 2: Using our module
	fmt.Println("\n=== Test 2: Using Our Module ===")
	testOurModule("password", "WRONG")
	time.Sleep(500 * time.Millisecond)
	testOurModule("admin1234", "CORRECT")
}

func testSimpleProtocol(password, label string) {
	fmt.Printf("\n--- Simple Protocol: %s password '%s' ---\n", label, password)

	conn, err := net.DialTimeout("tcp", "192.168.2.33:8728", 5*time.Second)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		return
	}
	defer conn.Close()

	// Build command exactly like protocol_check.go
	command := buildLoginCommand("admin", password)

	_, err = conn.Write(command)
	if err != nil {
		log.Printf("Write failed: %v", err)
		return
	}

	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("Read failed: %v", err)
		return
	}

	response := buf[:n]
	words := parseResponse(response)
	fmt.Printf("Simple protocol result: %v\n", words)
}

func testOurModule(password, label string) {
	fmt.Printf("\n--- Our Module: %s password '%s' ---\n", label, password)

	module := v6.NewMikrotikV6Module()
	module.Initialize("192.168.2.33", "admin", nil)

	success, err := module.Authenticate(context.Background(), password)
	module.Close()

	if err != nil {
		fmt.Printf("Module result: ERROR - %v\n", err)
	} else if success {
		fmt.Printf("Module result: SUCCESS\n")
	} else {
		fmt.Printf("Module result: FAILED\n")
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
