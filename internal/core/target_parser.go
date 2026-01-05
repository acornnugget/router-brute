package core

import (
	"bufio"
	"fmt"
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
// Examples:
//
//	"192.168.1.1" - IP only, uses defaults for username, port, command
//	"admin:192.168.1.1" - username and IP
//	"admin:192.168.1.1:8728" - username, IP, and port
//	"admin:192.168.1.1:8728:/login" - username, IP, port, and command
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

	// Use explicit field handling based on number of parts
	target := &Target{
		Username: "admin", // default
		Port:     p.defaultPort,
		Command:  p.defaultCommand,
	}

	switch len(parts) {
	case 1:
		// Only IP specified
		target.IP = parts[0]
	case 2:
		// Username and IP
		target.Username = parts[0]
		target.IP = parts[1]
	case 3:
		// Username, IP, and Port
		target.Username = parts[0]
		target.IP = parts[1]
		if port, err := strconv.Atoi(parts[2]); err == nil {
			target.Port = port
		} else {
			zlog.Warn().Str("port", parts[2]).Err(err).Msg("Invalid port, using default")
		}
	case 4:
		// Username, IP, Port, and Command
		target.Username = parts[0]
		target.IP = parts[1]
		if port, err := strconv.Atoi(parts[2]); err == nil {
			target.Port = port
		} else {
			zlog.Warn().Str("port", parts[2]).Err(err).Msg("Invalid port, using default")
		}
		target.Command = parts[3]
	default:
		zlog.Warn().Str("line", line).Int("parts", len(parts)).Msg("Invalid target format: too many fields")
		return nil, fmt.Errorf("invalid target format: %s", line)
	}

	// Validate IP is not empty
	if target.IP == "" {
		zlog.Warn().Str("line", line).Msg("Target IP cannot be empty")
		return nil, fmt.Errorf("target IP cannot be empty: %s", line)
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
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			zlog.Warn().Msg("Failed to close target file")
		}
	}(file)

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
