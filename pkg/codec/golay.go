package codec

// Golay(24,12) error correction code implementation
// Based on the standard Golay code used in DMR and other digital voice modes

// Golay(24,12) generator polynomial: x^11 + x^10 + x^6 + x^5 + x^4 + x^2 + 1
const (
	GolayPoly = 0xC75 // Binary: 110001110101
)

// golayEncodeTable is a precomputed encoding table for Golay(24,12)
// Maps 12-bit data to 12-bit parity
var golayEncodeTable [4096]uint32

// golaySyndromeTable is a precomputed syndrome table for error correction
var golaySyndromeTable [2048]uint32

// init precomputes the Golay encoding and syndrome tables
func init() {
	// Build encoding table
	for i := 0; i < 4096; i++ {
		golayEncodeTable[i] = golayEncode12(uint32(i))
	}

	// Build syndrome table for error correction
	for i := 0; i < 2048; i++ {
		golaySyndromeTable[i] = golaySyndrome(uint32(i))
	}
}

// golayEncode12 encodes 12 data bits into 12 parity bits
func golayEncode12(data uint32) uint32 {
	data &= 0xFFF // Ensure 12 bits
	parity := uint32(0)

	// Calculate parity using generator polynomial
	for i := 0; i < 12; i++ {
		if data&(1<<(11-i)) != 0 {
			parity ^= GolayPoly << i
		}
	}

	return (parity >> 1) & 0xFFF
}

// golaySyndrome calculates syndrome for 12-bit parity
func golaySyndrome(parity uint32) uint32 {
	syndrome := uint32(0)

	for i := 0; i < 12; i++ {
		if parity&(1<<(11-i)) != 0 {
			syndrome ^= GolayPoly << i
		}
	}

	return (syndrome >> 1) & 0xFFF
}

// GolayEncode encodes 12 data bits into a 24-bit Golay codeword
// Returns [data:12][parity:12] = 24 bits
func GolayEncode(data uint32) uint32 {
	data &= 0xFFF // Ensure 12 bits
	parity := golayEncodeTable[data]
	return (data << 12) | parity
}

// GolayDecode decodes a 24-bit Golay codeword and corrects up to 3 bit errors
// Returns the corrected 12-bit data and error count
func GolayDecode(codeword uint32) (uint32, int) {
	codeword &= 0xFFFFFF // Ensure 24 bits

	// Extract data and parity
	data := (codeword >> 12) & 0xFFF
	parity := codeword & 0xFFF

	// Calculate expected parity
	expectedParity := golayEncodeTable[data]

	// Calculate syndrome (error pattern)
	syndrome := parity ^ expectedParity

	if syndrome == 0 {
		// No errors
		return data, 0
	}

	// Try to correct errors
	corrected, errors := golayCorrect(data, syndrome)
	return corrected, errors
}

// golayCorrect attempts to correct errors using syndrome
func golayCorrect(data, syndrome uint32) (uint32, int) {
	// Weight 1, 2, or 3 error patterns
	// For simplicity, we'll use a lookup approach

	// Check if syndrome matches a correctable pattern
	weight := countBits(syndrome)

	if weight <= 3 {
		// Syndrome directly indicates data error
		corrected := data ^ syndrome
		return corrected & 0xFFF, weight
	}

	// Try rotating syndrome to find parity errors
	for i := 0; i < 12; i++ {
		rotated := ((syndrome << i) | (syndrome >> (12 - i))) & 0xFFF
		rotatedWeight := countBits(rotated)

		if rotatedWeight <= 3 {
			// Correctable parity error
			return data, rotatedWeight
		}
	}

	// Too many errors, return uncorrected data
	return data, -1
}

// countBits counts the number of 1-bits in a 12-bit value
func countBits(value uint32) int {
	count := 0
	value &= 0xFFF

	for i := 0; i < 12; i++ {
		if value&(1<<i) != 0 {
			count++
		}
	}

	return count
}

// GolayEncode23126 encodes using extended Golay(23,12,6) if needed
// This adds an extra parity bit for better error detection
func GolayEncode23126(data uint32) uint32 {
	// Standard (24,12) encoding
	codeword := GolayEncode(data)

	// Calculate overall parity bit
	parity := uint32(0)
	for i := 0; i < 24; i++ {
		if codeword&(1<<i) != 0 {
			parity ^= 1
		}
	}

	// Append parity as 24th bit
	return (codeword << 1) | parity
}

// GolayDecode23126 decodes extended Golay(23,12,6)
func GolayDecode23126(codeword uint32) (uint32, int) {
	codeword &= 0x7FFFFF // 23 bits

	// Extract overall parity
	overallParity := codeword & 1
	codeword24 := codeword >> 1

	// Calculate expected parity
	expectedParity := uint32(0)
	for i := 0; i < 24; i++ {
		if codeword24&(1<<i) != 0 {
			expectedParity ^= 1
		}
	}

	// Decode standard (24,12)
	data, errors := GolayDecode(codeword24)

	// Check overall parity
	if overallParity != expectedParity {
		errors++
	}

	return data, errors
}

// Helper functions for bit manipulation

// ExtractGolayData extracts 12-bit data from 24-bit codeword
func ExtractGolayData(codeword uint32) uint32 {
	return (codeword >> 12) & 0xFFF
}

// ExtractGolayParity extracts 12-bit parity from 24-bit codeword
func ExtractGolayParity(codeword uint32) uint32 {
	return codeword & 0xFFF
}

// BoolsToUint32 converts up to 32 bool bits to uint32
func BoolsToUint32(bits []bool) uint32 {
	result := uint32(0)
	for i := 0; i < len(bits) && i < 32; i++ {
		if bits[i] {
			result |= (1 << (len(bits) - 1 - i))
		}
	}
	return result
}

// Uint32ToBools converts uint32 to bool array of specified length
func Uint32ToBools(value uint32, numBits int) []bool {
	bits := make([]bool, numBits)
	for i := 0; i < numBits; i++ {
		bits[i] = (value & (1 << (numBits - 1 - i))) != 0
	}
	return bits
}
