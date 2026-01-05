package v6

import (
	"context"
	"testing"
	"time"
)

func TestMikrotikV6ModuleCreation(t *testing.T) {
	module := NewMikrotikV6Module()

	if module == nil {
		t.Fatal("Failed to create MikrotikV6Module")
	}

	if module.GetProtocolName() != "mikrotik-v6" {
		t.Errorf("Expected protocol name 'mikrotik-v6', got '%s'", module.GetProtocolName())
	}

	// Test initialization
	err := module.Initialize("192.168.1.1", "admin", map[string]interface{}{
		"port":    8729,
		"timeout": "5s",
	})

	if err != nil {
		t.Errorf("Failed to initialize module: %v", err)
	}

	if module.GetTarget() != "192.168.1.1" {
		t.Errorf("Expected target '192.168.1.1', got '%s'", module.GetTarget())
	}

	if module.GetUsername() != "admin" {
		t.Errorf("Expected username 'admin', got '%s'", module.GetUsername())
	}

	// Test that connection fails (expected since we don't have a real router)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = module.Connect(ctx)
	if err == nil {
		t.Error("Expected connection to fail (no real router), but it succeeded")
	}

	t.Logf("MikrotikV6Module creation test completed successfully")
}

func TestMikrotikV6ProtocolEncoding(t *testing.T) {
	module := NewMikrotikV6Module()

	// Test sentence encoding
	sentence := []string{"/login", "=name=admin", "=password=test123"}

	encoded, err := module.encodeSentence(sentence)
	if err != nil {
		t.Fatalf("Failed to encode sentence: %v", err)
	}

	if len(encoded) == 0 {
		t.Error("Expected non-empty encoded data")
	}

	// Test word decoding
	decodedWords, err := module.decodeWords(encoded)
	if err != nil {
		t.Fatalf("Failed to decode words: %v", err)
	}

	if len(decodedWords) != len(sentence) {
		t.Errorf("Expected %d decoded words, got %d", len(sentence), len(decodedWords))
	}

	// Check that the decoded words match the original
	for i, word := range sentence {
		if decodedWords[i] != word {
			t.Errorf("Word %d: expected '%s', got '%s'", i, word, decodedWords[i])
		}
	}

	t.Logf("Protocol encoding/decoding test completed successfully")
	t.Logf("Original sentence: %v", sentence)
	t.Logf("Encoded length: %d bytes", len(encoded))
	t.Logf("Decoded words: %v", decodedWords)
}
