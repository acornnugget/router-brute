package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nimda/router-brute/internal/interfaces"
	"github.com/nimda/router-brute/internal/modules"
	zlog "github.com/rs/zerolog/log"
)

// MikrotikV7RestModule implements the RouterOS v7 REST API protocol
// This uses HTTP/HTTPS with Basic Authentication instead of the binary API
type MikrotikV7RestModule struct {
	*modules.BaseRouterModule
	httpClient *http.Client
	baseURL    string
	useHTTPS   bool
	port       int
	timeout    time.Duration
}

// NewMikrotikV7RestModule creates a new Mikrotik RouterOS v7 REST API module
func NewMikrotikV7RestModule() *MikrotikV7RestModule {
	return &MikrotikV7RestModule{
		BaseRouterModule: modules.NewBaseRouterModule(),
		port:             80, // Default HTTP port
		useHTTPS:         false,
		timeout:          10 * time.Second,
	}
}

// GetProtocolName returns the protocol name
func (m *MikrotikV7RestModule) GetProtocolName() string {
	return "mikrotik-v7-rest"
}

// Initialize sets up the module with target information
func (m *MikrotikV7RestModule) Initialize(target, username string, options map[string]interface{}) error {
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

	if useHTTPS, ok := options["https"]; ok {
		if b, err := strconv.ParseBool(fmt.Sprintf("%v", useHTTPS)); err == nil {
			m.useHTTPS = b
			if m.useHTTPS && m.port == 80 {
				m.port = 443 // Default HTTPS port
			}
		}
	}

	// Build the base URL
	protocol := "http"
	if m.useHTTPS {
		protocol = "https"
	}

	// Remove any trailing slashes from target
	target = strings.TrimRight(target, "/")

	m.baseURL = fmt.Sprintf("%s://%s:%d/rest", protocol, target, m.port)

	zlog.Debug().Str("base_url", m.baseURL).Msg("RouterOS v7 REST API base URL")

	// Create HTTP client with timeout
	m.httpClient = &http.Client{
		Timeout: m.timeout,
	}

	return m.BaseRouterModule.Initialize(target, username, options)
}

// Connect establishes a connection to the Mikrotik router using REST API
// For REST API, connection is established on first request
func (m *MikrotikV7RestModule) Connect(ctx context.Context) error {
	if m.IsConnected() {
		zlog.Debug().Msg("Already connected, reusing existing connection")
		return nil
	}

	// For REST API, we don't need to establish a persistent connection
	// Connection is established on first HTTP request
	m.SetConnected(true)
	zlog.Debug().Msg("RouterOS v7 REST API connection ready")

	return nil
}

// Close cleans up the connection
func (m *MikrotikV7RestModule) Close() error {
	if !m.IsConnected() {
		return nil
	}

	m.SetConnected(false)
	zlog.Debug().Msg("RouterOS v7 REST API connection closed")

	return nil
}

// Authenticate attempts to authenticate with the given password using RouterOS v7 REST API
func (m *MikrotikV7RestModule) Authenticate(ctx context.Context, password string) (bool, error) {
	// Debug: Check context
	if ctx == nil {
		zlog.Error().Msg("nil context received in Authenticate()")
	}

	zlog.Trace().Str("username", m.GetUsername()).Str("target", m.GetTarget()).Msg("Starting RouterOS v7 REST API authentication attempt")

	// Always disconnect and reconnect for each attempt to avoid session issues
	if m.IsConnected() {
		zlog.Trace().Msg("Closing existing connection for fresh authentication attempt")
		if err := m.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing v7 REST connection during reauthentication")
		}
	}

	if err := m.Connect(ctx); err != nil {
		zlog.Trace().Err(err).Msg("Connection failed during authentication")
		return false, err
	}

	// Test authentication by making a simple REST API call
	// We'll try to access a basic system resource endpoint
	testURL := fmt.Sprintf("%s/system/resource", m.baseURL)

	zlog.Trace().Str("test_url", testURL).Msg("Testing RouterOS v7 REST API authentication")

	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		zlog.Trace().Err(err).Msg("Failed to create HTTP request")
		return false, err
	}

	// Set Basic Authentication
	req.SetBasicAuth(m.GetUsername(), password)
	req.Header.Set("Accept", "application/json")

	zlog.Trace().Msg("Sending RouterOS v7 REST API request")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		zlog.Trace().Err(err).Msg("RouterOS v7 REST API request failed")
		// Check if this is an authentication failure
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "Unauthorized") {
			zlog.Trace().Msg("Authentication failed (expected)")
			return false, nil
		}
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing REST response body")
		}
	}()

	zlog.Trace().Int("status_code", resp.StatusCode).Msg("Received RouterOS v7 REST API response")

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized {
		zlog.Trace().Msg("RouterOS v7 REST API authentication failed (401 Unauthorized)")
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		zlog.Trace().Int("status_code", resp.StatusCode).Msg("RouterOS v7 REST API request failed with non-200 status")
		// Read response body to get error details
		body, _ := io.ReadAll(resp.Body)
		zlog.Trace().Str("response_body", string(body)).Msg("RouterOS v7 REST API error response")
		return false, errors.New("REST API request failed with status: " + strconv.Itoa(resp.StatusCode))
	}

	// Read and parse the response to verify it's valid JSON
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zlog.Trace().Err(err).Msg("Failed to read RouterOS v7 REST API response body")
		return false, err
	}

	// Try to parse as JSON to verify it's a valid response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		zlog.Trace().Err(err).Str("response_body", string(body)).Msg("Failed to parse RouterOS v7 REST API response as JSON")
		return false, errors.New("invalid REST API response format")
	}

	zlog.Trace().Interface("response_data", result).Msg("Successfully parsed RouterOS v7 REST API response")

	// If we get here, authentication was successful
	zlog.Trace().Msg("RouterOS v7 REST API authentication successful")
	return true, nil
}

// Ensure MikrotikV7RestModule implements the RouterModule interface
var _ interfaces.RouterModule = (*MikrotikV7RestModule)(nil)
