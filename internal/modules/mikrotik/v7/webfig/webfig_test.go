package webfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebDecode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "simple ASCII",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "with encoded nulls",
			input:    "hello\xc4\x80world",
			expected: "hello\x00world",
		},
		{
			name:     "only encoded nulls",
			input:    "\xc4\x80\xc4\x80\xc4\x80",
			expected: "\x00\x00\x00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := webDecode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReverseSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "empty slice",
			input:    []byte{},
			expected: []byte{},
		},
		{
			name:     "single element",
			input:    []byte{0x01},
			expected: []byte{0x01},
		},
		{
			name:     "even length",
			input:    []byte{0x01, 0x02, 0x03, 0x04},
			expected: []byte{0x04, 0x03, 0x02, 0x01},
		},
		{
			name:     "odd length",
			input:    []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			expected: []byte{0x05, 0x04, 0x03, 0x02, 0x01},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reverseSlice(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
