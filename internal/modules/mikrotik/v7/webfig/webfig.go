// WebFig protocol implementation for RouterOS v7
// Based on the reference implementation from vulncheck-oss/go-exploit
package webfig

import (
	"bytes"
	"crypto/rand"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	zlog "github.com/rs/zerolog/log"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/text/encoding/charmap"
)

// WebfigSession represents a WebFig session with encryption state
type WebfigSession struct {
	ID  uint32
	Seq int
	Rx  *rc4.Cipher
	Tx  *rc4.Cipher
}

// reverseSlice reverses a byte slice
func reverseSlice(slice []byte) []byte {
	copied := make([]byte, len(slice))
	copy(copied, slice)

	for i := range len(copied) / 2 {
		j := len(copied) - 1 - i
		copied[i], copied[j] = copied[j], copied[i]
	}

	return copied
}

// webEncode converts binary data to WebFig encoding format
func webEncode(data []byte) (string, error) {
	decoder := charmap.ISO8859_1.NewDecoder()
	decodedData, err := decoder.Bytes(data)
	if err != nil {
		return "", fmt.Errorf("failed to decode data: %w", err)
	}
	convertedData := bytes.ReplaceAll(decodedData, []byte{0}, []byte{0xc4, 0x80})

	return string(convertedData), nil
}

// webDecode converts WebFig encoding back to binary data
func webDecode(data string) string {
	decodedData := bytes.ReplaceAll([]byte(data), []byte{0xC4, 0x80}, []byte{0x00})
	latin1Encoder := charmap.ISO8859_1.NewEncoder()
	decodedData, _ = latin1Encoder.Bytes(decodedData)

	return string(decodedData)
}

// generateKeyPair generates Curve25519 key pair
type KeyPair struct {
	PrivateKey []byte
	PublicKey  []byte
}

func generateKeyPair() (*KeyPair, error) {
	// Generate private key
	privateKey := make([]byte, 32)
	_, err := rand.Read(privateKey)
	if err != nil {
		return nil, err
	}

	// Apply Curve25519 Donna tweaks
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Generate public key (requires reversing)
	publicKey, err := curve25519.X25519(reverseSlice(privateKey), curve25519.Basepoint)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// generateSharedKey generates shared key from private and public keys
func generateSharedKey(privateKey []byte, publicKey []byte) ([]byte, error) {
	sharedKey, err := curve25519.X25519(reverseSlice(privateKey), reverseSlice(publicKey))
	if err != nil {
		return nil, err
	}

	return reverseSlice(sharedKey), nil
}

// initRC4 initializes RC4 ciphers for session encryption
func initRC4(session *WebfigSession, sharedKey []byte) error {
	// Initialize receive cipher
	rxKey := string(sharedKey) +
		strings.Repeat("\x00", 40) +
		"On the client side, this is the receive key; on the server side, it is the send key." +
		strings.Repeat("\xf2", 40)
	rxSha := sha1.Sum([]byte(rxKey))
	rxFinal := rxSha[:16]
	var err error
	session.Rx, err = rc4.NewCipher(rxFinal)
	if err != nil {
		return err
	}

	// Initialize send cipher
	txKey := string(sharedKey) +
		strings.Repeat("\x00", 40) +
		"On the client side, this is the send key; on the server side, it is the receive key." +
		strings.Repeat("\xf2", 40)
	txSha := sha1.Sum([]byte(txKey))
	txFinal := txSha[:16]
	session.Tx, err = rc4.NewCipher(txFinal)
	if err != nil {
		return err
	}

	// Drop first 768 bytes as per RouterOS implementation
	drop768 := strings.Repeat("\x00", 768)
	dst := make([]byte, len(drop768))
	session.Rx.XORKeyStream(dst, []byte(drop768))
	session.Tx.XORKeyStream(dst, []byte(drop768))

	return nil
}

// sendPublicKey sends public key to establish encryption
func sendPublicKey(webfigURL string, publicKey []byte, httpClient *http.Client) (string, error) {
	// Build payload: 8 null bytes + reversed public key
	payload := make([]byte, 8)
	payload = append(payload, reverseSlice(publicKey)...)

	// Send POST request to jsproxy endpoint
	encodedPayload, err := webEncode(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode payload: %w", err)
	}
	resp, err := httpClient.Post(webfigURL, "application/octet-stream", bytes.NewReader([]byte(encodedPayload)))
	if err != nil {
		return "", fmt.Errorf("failed to send POST request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing webfig response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read and decode response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return webDecode(string(body)), nil
}

// NegotiateEncryption negotiates WebFig encryption with the router
func NegotiateEncryption(webfigURL string, session *WebfigSession, httpClient *http.Client) error {
	// Generate key pair
	keyPair, err := generateKeyPair()
	if err != nil {
		return err
	}

	// Send public key and get response
	response, err := sendPublicKey(webfigURL, keyPair.PublicKey, httpClient)
	if err != nil {
		return err
	}

	// Validate response size (should be 40 bytes)
	if len(response) != 40 {
		return fmt.Errorf("unexpected public key response size: %d", len(response))
	}

	// Extract session ID and server public key
	session.ID = binary.BigEndian.Uint32([]byte(response[0:4]))
	serverPubKey := []byte(response[8:])

	// Generate shared key
	sharedKey, err := generateSharedKey(keyPair.PrivateKey, serverPubKey)
	if err != nil {
		return err
	}

	// Initialize RC4 ciphers
	err = initRC4(session, sharedKey)
	if err != nil {
		return err
	}

	// Initialize sequence number
	session.Seq = 1

	return nil
}

// sendEncryptedMessage sends an encrypted M2 message
func sendEncryptedMessage(webfigURL string, msg *M2Message, session *WebfigSession, httpClient *http.Client) (*M2Message, error) {
	// Serialize M2 message
	m2Data := msg.Serialize()

	// Add M2 header and padding
	encryptedMsg := []byte("M2")
	encryptedMsg = append(encryptedMsg, m2Data...)
	encryptedMsg = append(encryptedMsg, []byte(strings.Repeat(" ", 8))...)

	// Encrypt the message
	encrypted := make([]byte, len(encryptedMsg))
	session.Tx.XORKeyStream(encrypted, encryptedMsg)

	// Create header: [4 bytes ID][4 bytes sequence]
	idHeader := make([]byte, 4)
	seqHeader := make([]byte, 4)
	binary.BigEndian.PutUint32(idHeader, session.ID)
	binary.BigEndian.PutUint32(seqHeader, uint32(session.Seq))

	// Update sequence number
	session.Seq += len(encryptedMsg)

	// Build final payload
	finalMsg := []byte{}
	finalMsg = append(finalMsg, idHeader...)
	finalMsg = append(finalMsg, seqHeader...)
	finalMsg = append(finalMsg, encrypted...)

	// Send POST request
	resp, err := httpClient.Post(webfigURL, "msg", bytes.NewReader(finalMsg))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			zlog.Trace().Err(err).Msg("Error closing webfig response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Skip header (first 8 bytes)
	if len(responseBody) < 8 {
		return nil, errors.New("response too short")
	}
	responseBody = responseBody[8:]

	// Decrypt the response
	dst := make([]byte, len(responseBody))
	session.Rx.XORKeyStream(dst, responseBody)

	// Remove padding (last 8 bytes)
	dst = dst[:len(dst)-8]

	// Parse the response as M2 message
	responseMsg := NewM2Message()
	if !ParseM2Message(dst, responseMsg) {
		return nil, errors.New("failed to parse M2 response")
	}

	return responseMsg, nil
}

// Login authenticates with the router using WebFig protocol
func Login(webfigURL string, username string, password string, session *WebfigSession, httpClient *http.Client) (bool, error) {
	// Create login message
	msg := NewM2Message()
	msg.AddString(1, []byte(username))
	msg.AddString(3, []byte(password))

	// Send encrypted login message
	respMsg, err := sendEncryptedMessage(webfigURL, msg, session, httpClient)
	if err != nil {
		return false, err
	}

	// Check for authentication success
	if len(respMsg.Strings) > 0 {
		if _, ok := respMsg.Strings[0x15]; ok {
			// Authentication successful
			return true, nil
		}
	}

	return false, nil
}
