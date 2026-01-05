package core

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	zlog "github.com/rs/zerolog/log"
)

// Target represents a single router target with optional configuration
type Target struct {
	Username string
	IP       string
	Port     int
	Command  string
}

// TargetParser handles parsing of target specifications from files
type TargetParser struct {
	defaultCommand string
	defaultPort    int
}

// NewTargetParser creates a new target parser with specified defaults
func NewTargetParser(defaultCommand string, defaultPort int) *TargetParser {
	return &TargetParser{
		defaultCommand: defaultCommand,
		defaultPort:    defaultPort,
	}
}

// ParseTargetLine parses a single line from target file
// Format: username:ip:port:command (all fields except IP are optional)
func (p *TargetParser) ParseTargetLine(line string) (*Target, error) {
	zlog.Trace().Str("line", line).Msg("Parsing target line")

	// Skip comments and empty lines
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		zlog.Trace().Msg("Skipping empty/comment line")
		return nil, nil
	}

	parts := strings.Split(line, ":")
	if len(parts) == 0 {
		zlog.Warn().Str("line", line).Msg("Empty line after split")
		return nil, nil
	}

	// Parse fields in correct order: username:ip:port:command
	// Handle edge case where first field might be empty
	if len(parts) > 0 && parts[0] == "" {
		// Skip empty first field (malformed line)
		parts = parts[1:]
	}

	if len(parts) == 0 {
		zlog.Warn().Str("line", line).Msg("Empty line after processing")
		return nil, nil
	}

	target := &Target{
		Username: "admin", // default
		IP:       parts[0], // First field is IP when no username specified
		Port:     p.defaultPort,
		Command:  p.defaultCommand,
	}

	// Parse optional fields - format is username:ip:port:command
	if len(parts) > 1 && parts[1] != "" {
		// If we have at least 2 parts, parts[0] is username, parts[1] is IP
		target.Username = parts[0]
		target.IP = parts[1]
		if len(parts) > 2 && parts[2] != "" {
			port, err := strconv.Atoi(parts[2])
			if err != nil {
				zlog.Warn().Str("port", parts[2]).Err(err).Msg("Invalid port, using default")
				target.Port = p.defaultPort
			} else {
				target.Port = port
			}
		}
		if len(parts) > 3 && parts[3] != "" {
			target.Command = parts[3]
		}
	} else {
		// Only IP specified, use defaults for other fields
		target.IP = parts[0]
	}

	zlog.Debug().Interface("target", target).Msg("Parsed target")
	return target, nil
}

// ParseTargetFile reads and parses a target file
func (p *TargetParser) ParseTargetFile(filePath string) ([]*Target, error) {
	zlog.Info().Str("file", filePath).Msg("Loading targets from file")

	file, err := os.Open(filePath)
	if err != nil {
		zlog.Error().Str("file", filePath).Err(err).Msg("Failed to open target file")
		return nil, err
	}
	defer file.Close()

	var targets []*Target
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		target, err := p.ParseTargetLine(line)
		if err != nil {
			zlog.Warn().
				Str("file", filePath).
				Int("line", lineNum).
				Str("content", line).
				Err(err).
				Msg("Error parsing target line")
			continue
		}
		if target != nil {
			targets = append(targets, target)
		}
	}

	if err := scanner.Err(); err != nil {
		zlog.Error().Str("file", filePath).Err(err).Msg("Error reading target file")
		return nil, err
	}

	zlog.Info().Int("count", len(targets)).Msg("Loaded targets")
	return targets, nil
}