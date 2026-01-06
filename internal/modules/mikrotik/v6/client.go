package v6

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nimda/router-brute/internal/modules/mikrotik/common"

	"github.com/nimda/router-brute/internal/interfaces"
	"github.com/nimda/router-brute/internal/modules"
	"github.com/nimda/router-brute/pkg/utils"
	zlog "github.com/rs/zerolog/log"
)

// MikrotikV6Module implements the RouterOS v6 API protocol
type MikrotikV6Module struct {
	*modules.BaseRouterModule
	mu             sync.Mutex // Protects conn and authentication operations
	conn           net.Conn
	port           int
	timeout        time.Duration
	attemptsOnConn int // Track attempts on current connection (max 4 before reconnect)
}

// NewMikrotikV6Module creates a new Mikrotik RouterOS v6 module
func NewMikrotikV6Module() *MikrotikV6Module {
	return &MikrotikV6Module{
		BaseRouterModule: modules.NewBaseRouterModule(),
		port:             8728, // Default RouterOS API port
		timeout:          10 * time.Second,
	}
}

// GetProtocolName returns the protocol name
func (m *MikrotikV6Module) GetProtocolName() string {
	return "mikrotik-v6"
}

// Initialize sets up the module with target information
func (m *MikrotikV6Module) Initialize(target, username string, options map[string]interface{}) error {
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

	return m.BaseRouterModule.Initialize(target, username, options)
}

// Connect establishes a connection to the Mikrotik router
func (m *MikrotikV6Module) Connect(ctx context.Context) error {
	if m.IsConnected() {
		return nil
	}

	// Debug: Check if context is nil
	if ctx == nil {
		zlog.Error().Msg("ERROR: nil context passed to Connect()")
		return errors.New("nil context")
	}

	address := fmt.Sprintf("%s:%d", m.GetTarget(), m.port)

	// Set up connection with timeout
	dialer := &net.Dialer{
		Timeout: m.timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return utils.NewConnectionError(m.GetTarget(), err)
	}

	m.conn = conn
	m.SetConnected(true)

	// Set read/write deadlines
	if err := m.conn.SetDeadline(time.Now().Add(m.timeout)); err != nil {
		if err := m.conn.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing v6 connection after deadline failure")
		}
		m.SetConnected(false)
		return err
	}

	return nil
}

// Close cleans up the connection
func (m *MikrotikV6Module) Close() error {
	if !m.IsConnected() {
		return nil
	}

	if m.conn != nil {
		if err := m.conn.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing v6 connection")
		}
	}
	m.SetConnected(false)
	m.conn = nil
	m.attemptsOnConn = 0 // Reset attempt counter
	return nil
}

// Authenticate attempts to authenticate with the given password
func (m *MikrotikV6Module) Authenticate(ctx context.Context, password string) (bool, error) {
	// Lock to prevent concurrent authentication attempts from racing on connection
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate context
	if ctx == nil {
		return false, fmt.Errorf("nil context received in Authenticate()")
	}

	// RouterOS v6 limit: 5 failed auth attempts per connection
	// After 5th attempt, router sends "!fatal too many commands before login" and closes connection
	// We reconnect after 4 attempts to stay safely under the limit and maximize connection reuse
	if m.IsConnected() && m.attemptsOnConn >= 4 {
		zlog.Debug().
			Str("target", m.GetTarget()).
			Int("attempts", m.attemptsOnConn).
			Msg("Reconnecting after reaching attempt limit")
		if err := m.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing v6 connection before reconnect")
		}
	}

	// Connect if not connected
	if !m.IsConnected() {
		if err := m.Connect(ctx); err != nil {
			return false, err
		}
		m.attemptsOnConn = 0 // Reset counter on new connection
	}

	// Increment attempt counter before sending login
	m.attemptsOnConn++

	// Send the command
	if err := m.sendLogin(m.GetUsername(), password); err != nil {
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
			return false, nil
		}

		// Unknown error - close connection to be safe
		if closeErr := m.Close(); closeErr != nil {
			zlog.Trace().Err(closeErr).Msg("Error closing connection after unknown error")
		}
		m.attemptsOnConn = 0
		return false, err
	}

	// If we get here, authentication was successful
	return true, nil
}

func (m *MikrotikV6Module) sendLogin(user string, password string) error {
	if m.conn == nil {
		return errors.New("not connected")
	}
	command := buildLoginCommand(user, password)
	zlog.Debug().
		Str("target", m.GetTarget()).
		Str("username", user).
		Str("password", password).
		Msg("Trying:")

	zlog.Trace().Bytes("command", command).Str("password", password).Msg("Sending password")

	_, err := m.conn.Write(command)
	if err != nil {
		zlog.Error().Err(err).Msg("Error sending login")
		return err
	}

	response, err := m.readResponse()
	if err != nil {
		return err
	}

	// Parse the response for tracing
	words, err := m.decodeWords(response)

	if err == nil {
		zlog.Trace().Strs("words", words).Msg("Received response, decoded")
	} else {
		zlog.Trace().Bytes("response", response).Msg("Received response, decoding error")
	}

	// Parse the response
	return m.parseResponse(response)
}

func buildLoginCommand(username, password string) []byte {
	var buf []byte
	buf = common.AppendLengthPrefixed(buf, "/login")
	buf = common.AppendLengthPrefixed(buf, "=name="+username)
	buf = common.AppendLengthPrefixed(buf, "=password="+password)
	buf = append(buf, 0x00)
	return buf
}

// encodeSentence encodes a sentence into the Mikrotik binary format
func (m *MikrotikV6Module) encodeSentence(sentence []string) ([]byte, error) {
	var buf []byte

	// Write each word with length prefix
	for _, word := range sentence {
		wordBytes := []byte(word)
		if len(wordBytes) > 255 {
			return nil, errors.New("word too long")
		}

		// Write length (1 byte) + word
		buf = append(buf, byte(len(wordBytes)))
		buf = append(buf, wordBytes...)
	}

	// Add null terminator
	buf = append(buf, 0x00)

	return buf, nil
}

// readResponse reads a response from the Mikrotik router
func (m *MikrotikV6Module) readResponse() ([]byte, error) {
	buf := make([]byte, 4096)
	n, err := m.conn.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}

// parseResponse parses a Mikrotik response based on actual protocol analysis
func (m *MikrotikV6Module) parseResponse(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty response")
	}

	// Parse words from the binary response
	words, err := m.decodeWords(data)
	if err != nil {
		return err
	}

	zlog.Trace().Strs("words", words).Msg("Mikrotik response for authentication")

	// Check for explicit error responses first
	for _, word := range words {
		if word == "!trap" || word == "!fatal" {
			// This is definitely an authentication failure
			for _, w := range words {
				if strings.HasPrefix(w, "=message=") {
					message := strings.TrimPrefix(w, "=message=")
					return errors.New("authentication failed: " + message)
				}
			}
			return errors.New("authentication failed")
		}
	}

	// If we get here, there are no error indicators
	// Check if this is a success response
	// Mikrotik returns !done for successful logins
	for _, word := range words {
		if word == "!done" {
			// This is a successful authentication
			return nil
		}
	}

	// If we have =ret= but no !done, this might also be success
	for _, word := range words {
		if strings.HasPrefix(word, "=ret=") {
			return nil
		}
	}

	// Unexpected response pattern
	return errors.New("unexpected response: " + fmt.Sprintf("%v", words))
}

// decodeWords decodes words from binary data
func (m *MikrotikV6Module) decodeWords(data []byte) ([]string, error) {
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

// EncodeSentence exposes the encoding method for debugging
type DebugMikrotikV6 struct {
	*MikrotikV6Module
}

func (d *DebugMikrotikV6) EncodeSentence(sentence []string) ([]byte, error) {
	return d.encodeSentence(sentence)
}

func (d *DebugMikrotikV6) DecodeWords(data []byte) ([]string, error) {
	return d.decodeWords(data)
}

// Ensure MikrotikV6Module implements the RouterModule interface
var _ interfaces.RouterModule = (*MikrotikV6Module)(nil)
