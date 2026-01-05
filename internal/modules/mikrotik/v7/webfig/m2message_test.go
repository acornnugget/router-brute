package webfig

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestM2MessageCreation(t *testing.T) {
	// Create a new M2 message
	msg := NewM2Message()
	assert.NotNil(t, msg)

	// Add some test data
	msg.AddBool(1, true)
	msg.AddU32(2, 12345)
	msg.AddString(3, []byte("test string"))
	msg.AddU32Array(4, []uint32{100, 200, 300})

	// Serialize the message
	serialized := msg.Serialize()
	assert.NotNil(t, serialized)
	assert.True(t, len(serialized) > 0)

	// Create a new message to parse into
	parsedMsg := NewM2Message()

	// Parse the serialized data - message with data should be parseable
	success := ParseM2Message(serialized, parsedMsg)
	assert.True(t, success, "Message with data should be parseable")

	// Verify the parsed message
	assert.NotNil(t, parsedMsg)
}

func TestM2MessageWithEmptyData(t *testing.T) {
	// Create a new M2 message with no data
	msg := NewM2Message()
	assert.NotNil(t, msg)

	// Serialize the empty message
	serialized := msg.Serialize()
	assert.NotNil(t, serialized)

	// Create a new message to parse into
	parsedMsg := NewM2Message()

	// Parse the serialized data - empty message should return false
	success := ParseM2Message(serialized, parsedMsg)
	assert.False(t, success, "Empty message should not be parseable")

	// Verify the parsed message
	assert.NotNil(t, parsedMsg)
}

func TestM2MessageWithLargeData(t *testing.T) {
	// Create a new M2 message
	msg := NewM2Message()
	assert.NotNil(t, msg)

	// Add a large amount of data
	largeArray := make([]uint32, 256)
	for i := range largeArray {
		largeArray[i] = uint32(i)
	}
	msg.AddU32Array(1, largeArray)

	// Add some strings
	for i := 0; i < 10; i++ {
		msg.AddString(uint32(10+i), []byte("string-"+strconv.Itoa(i)))
	}

	// Serialize the message
	serialized := msg.Serialize()
	assert.NotNil(t, serialized)
	assert.True(t, len(serialized) > 0)

	// Create a new message to parse into
	parsedMsg := NewM2Message()

	// Parse the serialized data
	success := ParseM2Message(serialized, parsedMsg)
	assert.True(t, success)

	// Verify the parsed message
	assert.NotNil(t, parsedMsg)
}

func TestM2MessageMultipleTypes(t *testing.T) {
	// Create a new M2 message
	msg := NewM2Message()
	assert.NotNil(t, msg)

	// Add various types of data
	msg.AddBool(1, true)
	msg.AddBool(2, false)
	msg.AddU32(3, 42)
	msg.AddU32(4, 0)
	msg.AddU32(5, 0xFFFFFFFF)
	msg.AddString(6, []byte("hello world"))
	msg.AddU32Array(8, []uint32{1, 2, 3, 4, 5})

	// Serialize the message
	serialized := msg.Serialize()
	assert.NotNil(t, serialized)
	assert.True(t, len(serialized) > 0)

	// Create a new message to parse into
	parsedMsg := NewM2Message()

	// Parse the serialized data
	success := ParseM2Message(serialized, parsedMsg)
	assert.True(t, success)

	// Verify the parsed message
	assert.NotNil(t, parsedMsg)
}

func TestM2MessageErrorHandling(t *testing.T) {
	// Test error handling in M2 message functions

	// Test parsing invalid data
	parsedMsg := NewM2Message()
	success := ParseM2Message([]byte("invalid message"), parsedMsg)
	assert.False(t, success)

	// Test parsing empty data
	parsedMsg2 := NewM2Message()
	success2 := ParseM2Message([]byte(""), parsedMsg2)
	assert.False(t, success2)

	// Test parsing nil data
	parsedMsg3 := NewM2Message()
	success3 := ParseM2Message(nil, parsedMsg3)
	assert.False(t, success3)
}
