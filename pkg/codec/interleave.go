package codec

// Interleaving and bit mapping tables for YSF and DMR codecs
// Based on MMDVM_CM/YSF2DMR implementation by Jonathan Naylor G4KLX

// DMR bit mapping tables for AMBE frames in A, B, C slots
var (
	// DMR_A_TABLE maps AMBE bits to DMR slot A positions (24 bits)
	DMR_A_TABLE = []int{
		0, 4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44,
		48, 52, 56, 60, 64, 68, 1, 5, 9, 13, 17, 21,
	}

	// DMR_B_TABLE maps AMBE bits to DMR slot B positions (23 bits)
	DMR_B_TABLE = []int{
		25, 29, 33, 37, 41, 45, 49, 53, 57, 61, 65, 69,
		2, 6, 10, 14, 18, 22, 26, 30, 34, 38, 42,
	}

	// DMR_C_TABLE maps AMBE bits to DMR slot C positions (25 bits)
	DMR_C_TABLE = []int{
		46, 50, 54, 58, 62, 66, 70, 3, 7, 11, 15, 19, 23,
		27, 31, 35, 39, 43, 47, 51, 55, 59, 63, 67, 71,
	}

	// INTERLEAVE_TABLE_26_4 is used for 26x4 interleaving pattern
	INTERLEAVE_TABLE_26_4 = []int{
		0, 4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48, 52, 56, 60, 64, 68, 72, 76, 80, 84, 88, 92, 96, 100,
		1, 5, 9, 13, 17, 21, 25, 29, 33, 37, 41, 45, 49, 53, 57, 61, 65, 69, 73, 77, 81, 85, 89, 93, 97, 101,
		2, 6, 10, 14, 18, 22, 26, 30, 34, 38, 42, 46, 50, 54, 58, 62, 66, 70, 74, 78, 82, 86, 90, 94, 98, 102,
		3, 7, 11, 15, 19, 23, 27, 31, 35, 39, 43, 47, 51, 55, 59, 63, 67, 71, 75, 79, 83, 87, 91, 95, 99, 103,
	}

	// INTERLEAVE_TABLE_9_20 is used for 9x20 interleaving pattern
	INTERLEAVE_TABLE_9_20 = []int{
		0, 9, 18, 27, 36, 45, 54, 63, 72, 81, 90, 99, 108, 117, 126, 135, 144, 153, 162, 171,
		1, 10, 19, 28, 37, 46, 55, 64, 73, 82, 91, 100, 109, 118, 127, 136, 145, 154, 163, 172,
		2, 11, 20, 29, 38, 47, 56, 65, 74, 83, 92, 101, 110, 119, 128, 137, 146, 155, 164, 173,
		3, 12, 21, 30, 39, 48, 57, 66, 75, 84, 93, 102, 111, 120, 129, 138, 147, 156, 165, 174,
		4, 13, 22, 31, 40, 49, 58, 67, 76, 85, 94, 103, 112, 121, 130, 139, 148, 157, 166, 175,
		5, 14, 23, 32, 41, 50, 59, 68, 77, 86, 95, 104, 113, 122, 131, 140, 149, 158, 167, 176,
		6, 15, 24, 33, 42, 51, 60, 69, 78, 87, 96, 105, 114, 123, 132, 141, 150, 159, 168, 177,
		7, 16, 25, 34, 43, 52, 61, 70, 79, 88, 97, 106, 115, 124, 133, 142, 151, 160, 169, 178,
		8, 17, 26, 35, 44, 53, 62, 71, 80, 89, 98, 107, 116, 125, 134, 143, 152, 161, 170, 179,
	}

	// WHITENING_DATA is the PN9 sequence used for scrambling/descrambling
	WHITENING_DATA = []byte{
		0x93, 0xD7, 0x51, 0x21, 0x9C, 0x2F, 0x6C, 0xD0, 0xEF, 0x0F,
		0xF8, 0x3D, 0xF1, 0x73, 0x20, 0x94, 0xED, 0x1E, 0x7C, 0xD8,
	}

	// YSF VCH mapping - voice channel bit positions
	// YSF has 5 VCH (Voice Channels) per frame
	// Each VCH contains 104 bits (26 x 4 interleaving)
	YSF_VCH_BITS = 104

	// YSF DCH mapping - data channel bit positions
	YSF_DCH_BITS = 180
)

// DeinterleaveYSF deinterleaves YSF voice data using 26x4 pattern
func DeinterleaveYSF(input []bool) []bool {
	if len(input) != 104 {
		return input
	}

	output := make([]bool, 104)
	for i := 0; i < 104; i++ {
		output[i] = input[INTERLEAVE_TABLE_26_4[i]]
	}

	return output
}

// InterleaveYSF interleaves voice data for YSF using 26x4 pattern
func InterleaveYSF(input []bool) []bool {
	if len(input) != 104 {
		return input
	}

	output := make([]bool, 104)
	for i := 0; i < 104; i++ {
		output[INTERLEAVE_TABLE_26_4[i]] = input[i]
	}

	return output
}

// DeinterleaveDMR deinterleaves DMR voice data
// DMR uses different patterns for voice frame A, B, C
func DeinterleaveDMR(input []bool, frameType int) []bool {
	// DMR voice frame is 72 bits total (24 + 23 + 25)
	// Frame type determines which pattern to use (0=A, 1=B, 2=C)

	output := make([]bool, 72)
	var table []int

	switch frameType {
	case 0: // Frame A
		table = DMR_A_TABLE
	case 1: // Frame B
		table = DMR_B_TABLE
	case 2: // Frame C
		table = DMR_C_TABLE
	default:
		return input
	}

	// Extract bits according to the mapping table
	for i := 0; i < len(table) && i < len(output); i++ {
		if table[i] < len(input) {
			output[i] = input[table[i]]
		}
	}

	return output
}

// InterleaveDMR interleaves voice data for DMR
func InterleaveDMR(input []bool, frameType int) []bool {
	output := make([]bool, 72)
	var table []int

	switch frameType {
	case 0: // Frame A
		table = DMR_A_TABLE
	case 1: // Frame B
		table = DMR_B_TABLE
	case 2: // Frame C
		table = DMR_C_TABLE
	default:
		return input
	}

	// Insert bits according to the mapping table
	for i := 0; i < len(table) && i < len(input); i++ {
		if table[i] < len(output) {
			output[table[i]] = input[i]
		}
	}

	return output
}

// DescrambleYSF descrambles YSF data using PRNG sequence
func DescrambleYSF(input []bool) []bool {
	output := make([]bool, len(input))

	for i := 0; i < len(input); i++ {
		// XOR with whitening sequence
		byteIndex := (i / 8) % len(WHITENING_DATA)
		bitIndex := 7 - (i % 8)
		whiteningBit := (WHITENING_DATA[byteIndex] & (1 << bitIndex)) != 0

		output[i] = input[i] != whiteningBit // XOR operation
	}

	return output
}

// ScrambleYSF scrambles YSF data using PRNG sequence
func ScrambleYSF(input []bool) []bool {
	// Scrambling and descrambling are the same operation (XOR)
	return DescrambleYSF(input)
}

// DescrambletDMR descrambles DMR data using PN9 sequence
func DescrambletDMR(input []bool) []bool {
	output := make([]bool, len(input))

	for i := 0; i < len(input); i++ {
		// XOR with whitening sequence
		byteIndex := (i / 8) % len(WHITENING_DATA)
		bitIndex := 7 - (i % 8)
		whiteningBit := (WHITENING_DATA[byteIndex] & (1 << bitIndex)) != 0

		output[i] = input[i] != whiteningBit // XOR operation
	}

	return output
}

// ScrambleDMR scrambles DMR data using PN9 sequence
func ScrambleDMR(input []bool) []bool {
	// Scrambling and descrambling are the same operation (XOR)
	return DescrambletDMR(input)
}

// ConvertBitsToBytes converts a bool slice to byte slice
func ConvertBitsToBytes(bits []bool) []byte {
	numBytes := (len(bits) + 7) / 8
	bytes := make([]byte, numBytes)

	for i, bit := range bits {
		if bit {
			byteIndex := i / 8
			bitIndex := 7 - (i % 8)
			bytes[byteIndex] |= (1 << bitIndex)
		}
	}

	return bytes
}

// ConvertBytesToBits converts a byte slice to bool slice
func ConvertBytesToBits(bytes []byte, numBits int) []bool {
	bits := make([]bool, numBits)

	for i := 0; i < numBits && i < len(bytes)*8; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8)
		bits[i] = (bytes[byteIndex] & (1 << bitIndex)) != 0
	}

	return bits
}
