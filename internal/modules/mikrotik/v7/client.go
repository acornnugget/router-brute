package v7

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nimda/router-brute/internal/interfaces"
	"github.com/nimda/router-brute/internal/modules"
	"github.com/nimda/router-brute/internal/modules/mikrotik/common"
	"github.com/nimda/router-brute/internal/modules/mikrotik/v7/webfig"
	"github.com/nimda/router-brute/pkg/utils"
	zlog "github.com/rs/zerolog/log"
)

// MikrotikV7Module implements the RouterOS v7 API protocol
// RouterOS v7 uses WebFig protocol with encryption and M2 message format
type MikrotikV7Module struct {
	*modules.BaseRouterModule
	mu             sync.Mutex // Protects conn and authentication operations
	conn           net.Conn
	port           int
	timeout        time.Duration
	httpClient     *http.Client
	webfigURL      string
	useWebFig      bool
	attemptsOnConn int // Track attempts on current connection (max 4 before reconnect, binary mode only)
}

// NewMikrotikV7Module creates a new Mikrotik RouterOS v7 module
func NewMikrotikV7Module() *MikrotikV7Module {
	return &MikrotikV7Module{
		BaseRouterModule: modules.NewBaseRouterModule(),
		port:             8729, // Default RouterOS v7 API port
		timeout:          10 * time.Second,
	}
}

// GetProtocolName returns the protocol name
func (m *MikrotikV7Module) GetProtocolName() string {
	return "mikrotik-v7"
}

// Initialize sets up the module with target information
func (m *MikrotikV7Module) Initialize(target, username string, options map[string]interface{}) error {
	// Set default options
	if port, ok := options["port"]; ok {
		if p, err := strconv.Atoi(fmt.Sprintf("%v", port)); err == nil {
			m.port = p
		}
	}

	if timeout, ok := options["timeout"]; ok {
		if t, err := time.ParseDuration(fmt.Sprintf("%v", timeout)); err == nil {
			m.timeout = t
		}
	}

	// Check if we should use WebFig protocol (default for RouterOS v7)
	if useWebFig, ok := options["webfig"]; ok {
		if b, err := strconv.ParseBool(fmt.Sprintf("%v", useWebFig)); err == nil {
			m.useWebFig = b
		}
	} else {
		// Default to WebFig for RouterOS v7
		m.useWebFig = true
	}

	// If using WebFig, set up HTTP client and URL
	if m.useWebFig {
		protocol := "http"
		if m.port == 443 {
			protocol = "https"
		}
		m.webfigURL = fmt.Sprintf("%s://%s:%d/jsproxy", protocol, target, m.port)
		m.httpClient = &http.Client{
			Timeout: m.timeout,
		}
		zlog.Debug().Str("webfig_url", m.webfigURL).Msg("RouterOS v7 WebFig URL")
	}

	return m.BaseRouterModule.Initialize(target, username, options)
}

// Connect establishes a connection to the Mikrotik router using RouterOS v7 protocol
func (m *MikrotikV7Module) Connect(ctx context.Context) error {
	if m.IsConnected() {
		zlog.Debug().Msg("Already connected, reusing existing connection")
		return nil
	}

	// Debug: Check if context is nil
	if ctx == nil {
		zlog.Error().Msg("nil context passed to Connect()")
		return errors.New("nil context")
	}

	if m.useWebFig {
		// For WebFig, connection is established on first request
		zlog.Debug().Msg("RouterOS v7 WebFig connection ready")
		m.SetConnected(true)
		return nil
	}

	// Legacy binary API connection (for compatibility)
	address := fmt.Sprintf("%s:%d", m.GetTarget(), m.port)
	zlog.Trace().Str("address", address).Str("timeout", m.timeout.String()).Msg("Attempting RouterOS v7 binary connection")

	// Set up connection with timeout
	dialer := &net.Dialer{
		Timeout: m.timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		zlog.Trace().Err(err).Str("address", address).Msg("RouterOS v7 binary connection failed")
		return utils.NewConnectionError(m.GetTarget(), err)
	}

	m.conn = conn
	m.SetConnected(true)

	// Set read/write deadlines
	if err := m.conn.SetDeadline(time.Now().Add(m.timeout)); err != nil {
		zlog.Trace().Err(err).Msg("Failed to set connection deadline")
		if err := m.conn.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing v7 connection after deadline failure")
		}
		m.SetConnected(false)
		return err
	}

	zlog.Debug().Str("address", address).Msg("Connected using RouterOS v7 binary protocol")
	zlog.Trace().Str("local_addr", conn.LocalAddr().String()).Str("remote_addr", conn.RemoteAddr().String()).Msg("RouterOS v7 binary connection established")

	return nil
}

// Close cleans up the connection
func (m *MikrotikV7Module) Close() error {
	if !m.IsConnected() {
		return nil
	}

	if m.conn != nil {
		if err := m.conn.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing v7 connection")
		}
	}
	m.SetConnected(false)
	m.conn = nil
	m.attemptsOnConn = 0 // Reset attempt counter

	zlog.Debug().Msg("Connection closed")

	return nil
}

// Authenticate attempts to authenticate with the given password using RouterOS v7 protocol
func (m *MikrotikV7Module) Authenticate(ctx context.Context, password string) (bool, error) {
	// Lock to prevent concurrent authentication attempts from racing on connection
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate context
	if ctx == nil {
		return false, fmt.Errorf("nil context received in Authenticate()")
	}

	zlog.Trace().Str("username", m.GetUsername()).Str("target", m.GetTarget()).Msg("Starting RouterOS v7 authentication attempt")

	// For WebFig, each HTTP request is independent - no connection reuse needed
	if m.useWebFig {
		if err := m.Connect(ctx); err != nil {
			zlog.Trace().Err(err).Msg("Connection failed during authentication")
			return false, err
		}
		return m.authenticateWebFig(ctx, password)
	}

	// For binary API, reuse connection up to 4 attempts to stay under the 5-attempt limit
	// (RouterOS v7 binary API likely has similar limits to v6)
	if m.IsConnected() && m.attemptsOnConn >= 4 {
		zlog.Debug().
			Str("target", m.GetTarget()).
			Int("attempts", m.attemptsOnConn).
			Msg("Reconnecting after reaching attempt limit")
		if err := m.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing v7 connection before reconnect")
		}
	}

	// Connect if not connected
	if !m.IsConnected() {
		if err := m.Connect(ctx); err != nil {
			zlog.Trace().Err(err).Msg("Connection failed during authentication")
			return false, err
		}
		m.attemptsOnConn = 0 // Reset counter on new connection
	}

	// Increment attempt counter (binary mode only)
	m.attemptsOnConn++

	// Legacy binary API authentication (for compatibility)
	return m.authenticateBinary(ctx, password)
}

// authenticateWebFig authenticates using WebFig protocol
func (m *MikrotikV7Module) authenticateWebFig(ctx context.Context, password string) (bool, error) {
	zlog.Trace().Msg("Using RouterOS v7 WebFig authentication")

	// Create WebFig session
	session := &webfig.WebfigSession{}

	// Negotiate encryption
	if err := webfig.NegotiateEncryption(m.webfigURL, session, m.httpClient); err != nil {
		zlog.Trace().Err(err).Msg("WebFig encryption negotiation failed")
		// Check if this is an authentication failure vs connection error
		if strings.Contains(err.Error(), "unexpected status code") ||
			strings.Contains(err.Error(), "unexpected public key response") {
			zlog.Trace().Msg("WebFig authentication failed (expected)")
			return false, nil
		}
		return false, err
	}

	zlog.Trace().Msg("WebFig encryption negotiated successfully")

	// Perform login
	success, err := webfig.Login(m.webfigURL, m.GetUsername(), password, session, m.httpClient)
	if err != nil {
		zlog.Trace().Err(err).Msg("WebFig login failed")
		return false, err
	}

	if success {
		zlog.Trace().Msg("RouterOS v7 WebFig authentication successful")
		return true, nil
	}

	zlog.Trace().Msg("RouterOS v7 WebFig authentication failed")
	return false, nil
}

// authenticateBinary authenticates using legacy binary protocol (for compatibility)
func (m *MikrotikV7Module) authenticateBinary(ctx context.Context, password string) (bool, error) {
	zlog.Trace().Msg("Using RouterOS v7 binary authentication (legacy)")

	// Send the login command using RouterOS v7 protocol
	zlog.Trace().Str("username", m.GetUsername()).Msg("Sending RouterOS v7 login command")
	if err := m.sendLoginV7(m.GetUsername(), password); err != nil {
		// Check if this is a connection error (broken pipe, connection reset, EOF, etc.)
		errStr := err.Error()
		if strings.Contains(errStr, "broken pipe") ||
			strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "EOF") ||
			strings.Contains(errStr, "i/o timeout") ||
			strings.Contains(errStr, "connection refused") {
			// Connection is dead - close it and reset counter
			zlog.Debug().
				Str("target", m.GetTarget()).
				Err(err).
				Msg("Connection error detected, forcing reconnection")
			if closeErr := m.Close(); closeErr != nil {
				zlog.Trace().Err(closeErr).Msg("Error closing dead connection")
			}
			m.attemptsOnConn = 0
			return false, err
		}

		// Check if this is an authentication failure
		if strings.Contains(errStr, "invalid user name or password") ||
			strings.Contains(errStr, "!trap") ||
			strings.Contains(errStr, "!fatal") {
			zlog.Trace().Err(err).Msg("Authentication failed (expected)")
			return false, nil
		}

		// Unknown error - close connection to be safe
		zlog.Trace().Err(err).Msg("Authentication failed with unexpected error")
		if closeErr := m.Close(); closeErr != nil {
			zlog.Trace().Err(closeErr).Msg("Error closing connection after unknown error")
		}
		m.attemptsOnConn = 0
		return false, err
	}

	zlog.Trace().Msg("RouterOS v7 binary authentication successful")
	// If we get here, authentication was successful
	return true, nil
}

// sendLoginV7 sends a login command using RouterOS v7 protocol
func (m *MikrotikV7Module) sendLoginV7(user string, password string) error {
	if m.conn == nil {
		zlog.Trace().Msg("sendLoginV7 called but not connected")
		return errors.New("not connected")
	}

	// RouterOS v7 protocol implementation
	// Based on the analysis, the router is responding with v7 protocol format
	// Let's try a different approach - use the v6-style command but handle v7 responses

	command := buildLoginCommandV7(user, password)

	zlog.Trace().Int("command_length", len(command)).Msg("Built RouterOS v7 login command")
	zlog.Debug().Msg("Testing password with RouterOS v7 protocol")
	zlog.Trace().Msg("Sending RouterOS v7 login command")

	// Write the command to the connection
	zlog.Trace().Int("bytes_to_write", len(command)).Msg("Writing login command to router")
	n, err := m.conn.Write(command)
	if err != nil {
		zlog.Trace().Err(err).Int("bytes_written", n).Msg("RouterOS v7 command write failed")
		zlog.Debug().Err(err).Msg("Write failed")
		return err
	}
	zlog.Trace().Int("bytes_written", n).Msg("Successfully wrote login command")

	// Read the response
	zlog.Trace().Msg("Waiting for RouterOS v7 response")
	response, err := m.readResponse()
	if err != nil {
		zlog.Trace().Err(err).Msg("Failed to read RouterOS v7 response")
		return err
	}
	zlog.Trace().Int("response_length", len(response)).Msg("Received RouterOS v7 response")

	// Parse the response for tracing
	words, err := m.decodeWordsV7(response)
	if err != nil {
		zlog.Trace().Err(err).Interface("raw_response", response).Msg("Failed to decode RouterOS v7 response words")
	} else {
		zlog.Trace().Interface("response", words).Msg("Successfully decoded RouterOS v7 response")
	}

	// Parse the response
	zlog.Trace().Msg("Parsing RouterOS v7 response")
	return m.parseResponseV7(response)
}

// buildLoginCommandV7 builds a login command for RouterOS v7 protocol
func buildLoginCommandV7(username, password string) []byte {
	// RouterOS v7 uses a different authentication protocol than v6
	// The v7 protocol requires proper session establishment and challenge-response

	// For now, we'll try the basic format that should work with RouterOS v7
	// This is a simplified version that should work for basic authentication
	var buf []byte

	// RouterOS v7 login command format
	buf = common.AppendLengthPrefixed(buf, "/login")
	buf = common.AppendLengthPrefixed(buf, "=name="+username)
	buf = common.AppendLengthPrefixed(buf, "=password="+password)

	// Add the null terminator
	buf = append(buf, 0x00)

	return buf
}

// readResponse reads a response from the Mikrotik router
func (m *MikrotikV7Module) readResponse() ([]byte, error) {
	buf := make([]byte, 4096)
	n, err := m.conn.Read(buf)
	if err != nil {
		return nil, err
	}

	zlog.Trace().Int("bytes", n).Msg("Read from router")

	return buf[:n], nil
}

// parseResponseV7 parses a Mikrotik RouterOS v7 response
func (m *MikrotikV7Module) parseResponseV7(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty response")
	}

	// First, check for the specific RouterOS v7 failure pattern we observed
	if len(data) >= 7 && data[0] == 0x15 && data[1] == 0x00 && data[2] == 0x00 && data[3] == 0x00 {
		zlog.Debug().Hex("response", data).Msg("RouterOS v7 authentication failed (binary pattern)")
		return errors.New("authentication failed")
	}

	// Try to parse as RouterOS v7 format
	words, err := m.decodeWordsV7(data)
	if err != nil {
		// If v7 parsing fails, try v6 fallback
		zlog.Trace().Err(err).Msg("RouterOS v7 parsing failed, trying v6 fallback")
		words, err = m.decodeWordsV6Fallback(data)
		if err != nil {
			// If both fail, return the original error
			return err
		}
	}

	// Debug logging
	zlog.Debug().Interface("response", words).Msg("RouterOS v7 authentication response")

	// Check for success patterns first
	for _, word := range words {
		if word == "!done" {
			zlog.Info().Msg("✓ RouterOS v7 authentication successful")
			return nil
		}
		if strings.HasPrefix(word, "=ret=") {
			zlog.Info().Msg("✓ RouterOS v7 authentication successful (ret)")
			return nil
		}
	}

	// Check for failure patterns
	for _, word := range words {
		if word == "!trap" || word == "!fatal" {
			for _, w := range words {
				if strings.HasPrefix(w, "=message=") {
					message := strings.TrimPrefix(w, "=message=")
					return errors.New("authentication failed: " + message)
				}
			}
			return errors.New("authentication failed")
		}
	}

	// If we get here, we have an unexpected response
	return errors.New("unexpected RouterOS v7 response: " + fmt.Sprintf("%v", words))
}

// decodeWordsV7 decodes words from RouterOS v7 binary data
// RouterOS v7 uses a different format than v6 - it includes a 4-byte length header
func (m *MikrotikV7Module) decodeWordsV7(data []byte) ([]string, error) {
	if len(data) < 4 {
		return nil, errors.New("response too short for RouterOS v7 format")
	}

	// RouterOS v7 responses start with a 4-byte length header (big-endian)
	// The first byte is the total length, followed by response type
	responseType := data[0]

	// Check for RouterOS v7 specific response patterns
	if responseType == 0x15 { // This appears to be an error/failure response
		// For authentication failures, RouterOS v7 returns a specific pattern
		// We can detect this and return appropriate error
		return nil, errors.New("authentication failed")
	}

	// For successful responses, we expect different patterns
	// But since we're getting authentication failures, we can handle them here

	// Fallback to v6-style decoding for compatibility
	return m.decodeWordsV6Fallback(data)
}

// decodeWordsV6Fallback provides backward compatibility with v6-style decoding
func (m *MikrotikV7Module) decodeWordsV6Fallback(data []byte) ([]string, error) {
	var words []string
	i := 0

	for i < len(data) {
		if data[i] == 0x00 {
			// End of sentence
			break
		}

		// Read length
		length := int(data[i])
		i++

		// Read word
		if i+length > len(data) {
			return nil, errors.New("invalid word length")
		}

		word := string(data[i : i+length])
		words = append(words, word)
		i += length
	}

	return words, nil
}

// Ensure MikrotikV7Module implements the RouterModule interface
var _ interfaces.RouterModule = (*MikrotikV7Module)(nil)
