package v7

import (
	"context"
	"testing"
	"time"
)

func TestMikrotikV7ModuleCreation(t *testing.T) {
	module := NewMikrotikV7Module()

	if module == nil {
		t.Fatal("Failed to create MikrotikV7Module")
	}

	if module.GetProtocolName() != "mikrotik-v7" {
		t.Errorf("Expected protocol name 'mikrotik-v7', got '%s'", module.GetProtocolName())
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

	// Test that connection succeeds for WebFig (it doesn't actually connect until first request)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = module.Connect(ctx)
	if err != nil {
		t.Errorf("Expected WebFig connection to succeed, but got error: %v", err)
	}

	t.Logf("MikrotikV7Module creation test completed successfully")
}

func TestMikrotikV7ProtocolEncoding(t *testing.T) {
	module := NewMikrotikV7Module()

	// Test sentence encoding (using the same protocol as v6 for basic encoding)
	sentence := []string{"/login", "=name=admin", "=password=test123"}

	// Note: The v7 module doesn't expose encodeSentence directly, but we can test
	// the helper functions that are used internally
	var buf []byte
	buf = appendLengthPrefixed(buf, "/login")
	buf = appendLengthPrefixed(buf, "=name=admin")
	buf = appendLengthPrefixed(buf, "=password=test123")
	buf = append(buf, 0x00)

	if len(buf) == 0 {
		t.Error("Expected non-empty encoded data")
	}

	// Test word decoding
	decodedWords, err := module.decodeWordsV7(buf)
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
	t.Logf("Encoded length: %d bytes", len(buf))
	t.Logf("Decoded words: %v", decodedWords)
}
