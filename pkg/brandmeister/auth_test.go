package brandmeister

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

func TestCalculateResponse(t *testing.T) {
	password := "mypassword"
	challenge := uint32(0x12345678)

	// Prepend 32-bit pattern to password string
	challengeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(challengeBytes, challenge)

	expectedInput := append(challengeBytes, []byte(password)...)
	expectedHash := sha256.Sum256(expectedInput)

	hash := CalculateResponse(challenge, password)

	if !bytes.Equal(hash[:], expectedHash[:]) {
		t.Errorf("Expected hash %x, got %x", expectedHash, hash)
	}
}
