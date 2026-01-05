package core

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTargetLine_BasicIP(t *testing.T) {
	parser := NewTargetParser("default_cmd", 8728)
	target, err := parser.ParseTargetLine("192.168.1.1")

	assert.NoError(t, err)
	assert.NotNil(t, target)
	assert.Equal(t, "admin", target.Username)
	assert.Equal(t, "192.168.1.1", target.IP)
	assert.Equal(t, 8728, target.Port)
	assert.Equal(t, "default_cmd", target.Command)
}

func TestParseTargetLine_WithUsername(t *testing.T) {
	parser := NewTargetParser("default_cmd", 8728)
	target, err := parser.ParseTargetLine("custom_user:192.168.1.1")

	assert.NoError(t, err)
	assert.NotNil(t, target)
	assert.Equal(t, "custom_user", target.Username)
	assert.Equal(t, "192.168.1.1", target.IP)
	assert.Equal(t, 8728, target.Port)
	assert.Equal(t, "default_cmd", target.Command)
}

func TestParseTargetLine_WithUsernameAndPort(t *testing.T) {
	parser := NewTargetParser("default_cmd", 8728)
	target, err := parser.ParseTargetLine("admin:192.168.1.1:8729")

	assert.NoError(t, err)
	assert.NotNil(t, target)
	assert.Equal(t, "admin", target.Username)
	assert.Equal(t, "192.168.1.1", target.IP)
	assert.Equal(t, 8729, target.Port)
	assert.Equal(t, "default_cmd", target.Command)
}

func TestParseTargetLine_WithAllFields(t *testing.T) {
	parser := NewTargetParser("default_cmd", 8728)
	target, err := parser.ParseTargetLine("custom:192.168.1.1:8729:special_command")

	assert.NoError(t, err)
	assert.NotNil(t, target)
	assert.Equal(t, "custom", target.Username)
	assert.Equal(t, "192.168.1.1", target.IP)
	assert.Equal(t, 8729, target.Port)
	assert.Equal(t, "special_command", target.Command)
}

func TestParseTargetLine_EmptyFields(t *testing.T) {
	parser := NewTargetParser("default_cmd", 8728)
	target, err := parser.ParseTargetLine(":::192.168.1.1")

	// Should return error because IP cannot be empty
	assert.Error(t, err)
	assert.Nil(t, target)
	assert.Contains(t, err.Error(), "target IP cannot be empty")
}

func TestParseTargetLine_InvalidPort(t *testing.T) {
	parser := NewTargetParser("default_cmd", 8728)
	target, err := parser.ParseTargetLine("admin:192.168.1.1:invalid_port")

	assert.NoError(t, err)
	assert.NotNil(t, target)
	assert.Equal(t, "admin", target.Username)
	assert.Equal(t, "192.168.1.1", target.IP)
	assert.Equal(t, 8728, target.Port) // Should use default due to invalid port
	assert.Equal(t, "default_cmd", target.Command)
}

func TestParseTargetLine_CommentLine(t *testing.T) {
	parser := NewTargetParser("default_cmd", 8728)
	target, err := parser.ParseTargetLine("# This is a comment")

	assert.NoError(t, err)
	assert.Nil(t, target) // Should return nil for comment lines
}

func TestParseTargetLine_EmptyLine(t *testing.T) {
	parser := NewTargetParser("default_cmd", 8728)
	target, err := parser.ParseTargetLine("")

	assert.NoError(t, err)
	assert.Nil(t, target) // Should return nil for empty lines
}

func TestParseTargetLine_WhitespaceOnly(t *testing.T) {
	parser := NewTargetParser("default_cmd", 8728)
	target, err := parser.ParseTargetLine("   \t\n")

	assert.NoError(t, err)
	assert.Nil(t, target) // Should return nil for whitespace-only lines
}

func TestParseTargetFile(t *testing.T) {
	// Create temporary test file
	testContent := `# Test targets file
192.168.1.1
admin:192.168.1.2:8729
custom:10.0.0.1:8080:special
# Another comment
172.16.0.1

  # Empty line with spaces
invalid_port:192.168.1.3:abc
`

	tmpFile, err := os.CreateTemp("", "test_targets_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(testContent)
	require.NoError(t, err)
	tmpFile.Close()

	parser := NewTargetParser("default_cmd", 8728)
	targets, err := parser.ParseTargetFile(tmpFile.Name())

	assert.NoError(t, err)
	assert.Len(t, targets, 5) // Should parse 5 valid targets

	// Verify first target
	assert.Equal(t, "admin", targets[0].Username)
	assert.Equal(t, "192.168.1.1", targets[0].IP)
	assert.Equal(t, 8728, targets[0].Port)
	assert.Equal(t, "default_cmd", targets[0].Command)

	// Verify second target
	assert.Equal(t, "admin", targets[1].Username)
	assert.Equal(t, "192.168.1.2", targets[1].IP)
	assert.Equal(t, 8729, targets[1].Port)
	assert.Equal(t, "default_cmd", targets[1].Command)

	// Verify third target (all fields)
	assert.Equal(t, "custom", targets[2].Username)
	assert.Equal(t, "10.0.0.1", targets[2].IP)
	assert.Equal(t, 8080, targets[2].Port)
	assert.Equal(t, "special", targets[2].Command)

	// Verify fourth target
	assert.Equal(t, "admin", targets[3].Username)
	assert.Equal(t, "172.16.0.1", targets[3].IP)
	assert.Equal(t, 8728, targets[3].Port)
	assert.Equal(t, "default_cmd", targets[3].Command)

	// Verify fifth target (invalid port should default)
	assert.Equal(t, "invalid_port", targets[4].Username)
	assert.Equal(t, "192.168.1.3", targets[4].IP)
	assert.Equal(t, 8728, targets[4].Port) // Should default due to invalid port
	assert.Equal(t, "default_cmd", targets[4].Command)
}

func TestParseTargetFile_EmptyFile(t *testing.T) {
	// Create empty temporary file
	tmpFile, err := os.CreateTemp("", "empty_targets_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	parser := NewTargetParser("default_cmd", 8728)
	targets, err := parser.ParseTargetFile(tmpFile.Name())

	assert.NoError(t, err)
	assert.Len(t, targets, 0) // Empty file should return empty slice
}

func TestParseTargetFile_NonExistentFile(t *testing.T) {
	parser := NewTargetParser("default_cmd", 8728)
	targets, err := parser.ParseTargetFile("nonexistent_file.txt")

	assert.Error(t, err)
	assert.Nil(t, targets)
}

func TestParseTargetFile_OnlyComments(t *testing.T) {
	testContent := `# Just comments
# More comments
# No actual targets
`

	tmpFile, err := os.CreateTemp("", "comments_only_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(testContent)
	require.NoError(t, err)
	tmpFile.Close()

	parser := NewTargetParser("default_cmd", 8728)
	targets, err := parser.ParseTargetFile(tmpFile.Name())

	assert.NoError(t, err)
	assert.Len(t, targets, 0) // Only comments should return empty slice
}

func TestNewTargetParser(t *testing.T) {
	parser := NewTargetParser("test_command", 1234)

	assert.NotNil(t, parser)
	assert.Equal(t, "test_command", parser.defaultCommand)
	assert.Equal(t, 1234, parser.defaultPort)
}
