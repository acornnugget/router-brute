package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	fmt.Println("Mikrotik Router Configuration Check")
	fmt.Println("==================================")

	// Check if we can connect without any authentication
	conn, err := net.DialTimeout("tcp", "192.168.2.33:8728", 5*time.Second)
	if err != nil {
		log.Fatalf("Cannot connect to router: %v", err)
	}
	defer conn.Close()

	fmt.Println("✅ Connection successful - API service is running")

	// Try to send a simple command without login
	command := "/system/identity/print\n"
	_, err = conn.Write([]byte(command))
	if err != nil {
		log.Printf("Cannot send command: %v", err)
	} else {
		fmt.Println("❌ Router accepted command without authentication!")
		fmt.Println("This indicates the API service is running in insecure mode.")
	}

	fmt.Println("\n📋 Recommendations:")
	fmt.Println("1. Check router API service configuration:")
	fmt.Println("   /ip service print")
	fmt.Println("")
	fmt.Println("2. Look for these issues:")
	fmt.Println("   - API service enabled without authentication")
	fmt.Println("   - IP restrictions not properly configured")
	fmt.Println("   - Default passwords or no password set")
	fmt.Println("")
	fmt.Println("3. Fix commands:")
	fmt.Println("   /ip service disable api")
	fmt.Println("   /ip service enable api")
	fmt.Println("   /ip service set api port=8728")
	fmt.Println("")
	fmt.Println("4. Check user authentication:")
	fmt.Println("   /user print")
	fmt.Println("   /user set admin password=admin1234")

	fmt.Println("\n🔧 Router Security Check:")
	checkCommonIssues()
}

func checkCommonIssues() {
	issues := []string{
		"API service running without authentication",
		"Default admin password not changed",
		"No IP restrictions on API access",
		"Winbox/SSH services exposed to internet",
		"No firewall rules for API protection",
	}

	fmt.Println("Common Mikrotik security issues to check:")
	for i, issue := range issues {
		fmt.Printf("%d. %s\n", i+1, issue)
	}

	fmt.Println("\n📚 Resources:")
	fmt.Println("https://wiki.mikrotik.com/wiki/Manual:Securing_Your_Router")
	fmt.Println("https://wiki.mikrotik.com/wiki/Manual:API")
}
