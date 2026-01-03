# Step 7: RouterOS v7 Module Implementation

## Objective
Implement support for Mikrotik RouterOS v7 protocol with enhanced security features and updated API.

## Research Findings

### Key Differences from RouterOS v6

1. **Protocol Enhancements**:
   - Port 8729 as primary (8728 for compatibility)
   - Enhanced authentication with session tokens
   - Improved error handling and responses
   - Native SSL/TLS support

2. **API Changes**:
   - Version negotiation in handshake
   - Extended command structure
   - More detailed error messages
   - Session-based authentication

3. **Security Improvements**:
   - Stronger encryption options
   - Certificate validation
   - Better session management
   - Client identification requirements

## Implementation Plan

### Phase 1: Module Structure (1 hour)
1. Create `internal/modules/mikrotik/v7/` directory
2. Set up basic module skeleton
3. Implement interface compliance
4. Add version-specific constants

### Phase 2: Core Protocol (2-3 hours)
1. Implement v7 binary protocol encoding
2. Create enhanced response parsing
3. Add version negotiation
4. Implement session management

### Phase 3: SSL/TLS Support (1-2 hours)
1. Add SSL connection handling
2. Implement certificate validation
3. Add insecure mode option
4. Test SSL handshake

### Phase 4: Authentication Logic (2 hours)
1. Implement v7 login command structure
2. Add session token handling
3. Implement keep-alive mechanism
4. Test authentication flow

### Phase 5: Integration (1 hour)
1. Connect to core engine
2. Add CLI support
3. Implement configuration options
4. Test end-to-end functionality

### Phase 6: Testing (2 hours)
1. Unit tests for protocol encoding
2. Mock tests for responses
3. Integration tests with engine
4. Error handling validation

## Technical Implementation Details

### Protocol Encoding
```go
func encodeV7Sentence(sentence []string) ([]byte, error) {
    // Add version identifier
    sentence = append([]string{"=version=7"}, sentence...)
    
    // Standard length-prefixed encoding
    var buf []byte
    for _, word := range sentence {
        wordBytes := []byte(word)
        if len(wordBytes) > 255 {
            return nil, errors.New("word too long")
        }
        buf = append(buf, byte(len(wordBytes)))
        buf = append(buf, wordBytes...)
    }
    
    buf = append(buf, 0x00) // Null terminator
    return buf, nil
}
```

### SSL Connection
```go
func connectWithSSL(ctx context.Context, target string, port int) (net.Conn, error) {
    address := fmt.Sprintf("%s:%d", target, port)
    
    config := &tls.Config{
        InsecureSkipVerify: false, // Proper validation
        MinVersion:         tls.VersionTLS12,
    }
    
    dialer := &net.Dialer{Timeout: 10 * time.Second}
    return tls.DialWithDialer(dialer, "tcp", address, config)
}
```

### Session Management
```go
type SessionManager struct {
    sessionID   string
    lastUsed    time.Time
    timeout     time.Duration
    keepAlive   bool
}

func (sm *SessionManager) isValid() bool {
    return time.Since(sm.lastUsed) < sm.timeout
}
```

## Configuration Options

### CLI Flags for v7
```
--port      RouterOS v7 API port (default: 8729)
--ssl       Enable SSL/TLS connection
--insecure  Skip certificate verification (not recommended)
--version   Force protocol version (auto-detect by default)
```

### Module Initialization
```go
module := v7.NewMikrotikV7Module()
module.Initialize("192.168.1.1", "admin", map[string]interface{}{
    "port":     8729,
    "ssl":      true,
    "insecure": false,
    "timeout":  "15s",
})
```

## Testing Strategy

### Test Cases Required
1. **Protocol Encoding**: Verify v7 format compliance
2. **SSL Handshake**: Test encrypted connections
3. **Session Management**: Validate token handling
4. **Error Responses**: Test new error formats
5. **Backward Compatibility**: Test v6 compatibility mode

### Mock Responses for Testing
```go
// Successful v7 login response
mockSuccessResponse := []byte{
    0x06, '=','r','e','t','=',  // =ret=
    0x08, 's','e','s','s','i','o','n',  // session
    0x00,  // Null terminator
}

// v7 error response
mockErrorResponse := []byte{
    0x05, '!','t','r','a','p',  // !trap
    0x0B, '=','m','e','s','s','a','g','e','=','i','n','v','a','l','i','d',  // =message=invalid
    0x00,  // Null terminator
}
```

## Expected Challenges

1. **SSL Certificate Handling**: Managing self-signed certificates
2. **Session Timeout**: Proper session lifecycle management
3. **Version Detection**: Auto-detection between v6 and v7
4. **Error Parsing**: Handling new error message formats
5. **Performance**: SSL overhead vs plain TCP

## Success Criteria

- [ ] Module compiles without errors
- [ ] All unit tests pass
- [ ] Integration with core engine works
- [ ] SSL connections successful
- [ ] Session management functional
- [ ] Backward compatibility maintained
- [ ] Documentation complete

## Timeline Estimate
- **Total**: 8-10 hours
- **Priority**: Medium (after v6 validation)
- **Dependencies**: Core engine stability, v6 module validation

## Next Steps
1. Validate v6 implementation with real device
2. Document any v6 protocol quirks found
3. Begin v7 implementation based on findings
4. Test both versions side-by-side