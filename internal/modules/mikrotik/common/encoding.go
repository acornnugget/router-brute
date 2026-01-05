package common

// AppendLengthPrefixed adds a length-prefixed word to the buffer.
// This is used in Mikrotik RouterOS binary protocol encoding.
// The length is stored as a single byte (max 255 bytes).
func AppendLengthPrefixed(buf []byte, word string) []byte {
	wordBytes := []byte(word)
	if len(wordBytes) > 255 {
		wordBytes = wordBytes[:255]
	}
	buf = append(buf, byte(len(wordBytes)))
	buf = append(buf, wordBytes...)
	return buf
}
