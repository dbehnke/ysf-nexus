package brandmeister

import (
	"crypto/sha256"
	"encoding/binary"
)

// CalculateResponse generates the SHA-256 hash required for BrandMeister authentication.
// It prepends the 32-bit challenge (little-endian) to the password and hashes the result.
func CalculateResponse(challenge uint32, password string) [32]byte {
	challengeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(challengeBytes, challenge)

	input := append(challengeBytes, []byte(password)...)
	return sha256.Sum256(input)
}
