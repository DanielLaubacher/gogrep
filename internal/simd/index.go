package simd

import (
	"bytes"
	"math/bits"

	"simd/archsimd"
)

// Index returns the index of the first occurrence of pattern in data, or -1 if not present.
// Uses the SIMD-friendly Horspool algorithm: broadcasts the first and last bytes of the pattern,
// compares 32 candidate positions simultaneously using AVX2, then verifies middle bytes only
// for positions where both first and last bytes match.
func Index(data, pattern []byte) int {
	plen := len(pattern)
	switch {
	case plen == 0:
		return 0
	case plen == 1:
		return bytes.IndexByte(data, pattern[0])
	case plen > len(data):
		return -1
	}

	first := archsimd.BroadcastUint8x32(pattern[0])
	last := archsimd.BroadcastUint8x32(pattern[plen-1])

	i := 0
	limit := len(data) - plen + 1

	for i+32 <= limit {
		blockFirst := archsimd.LoadUint8x32Slice(data[i:])
		blockLast := archsimd.LoadUint8x32Slice(data[i+plen-1:])

		mFirst := blockFirst.Equal(first)
		mLast := blockLast.Equal(last)
		b := mFirst.And(mLast).ToBits()

		for b != 0 {
			j := bits.TrailingZeros32(b)
			candidate := data[i+j : i+j+plen]
			if plen <= 2 || bytes.Equal(candidate[1:plen-1], pattern[1:plen-1]) {
				archsimd.ClearAVXUpperBits()
				return i + j
			}
			b &= b - 1
		}

		i += 32
	}

	// Scalar tail
	idx := bytes.Index(data[i:], pattern)
	archsimd.ClearAVXUpperBits()
	if idx >= 0 {
		return i + idx
	}
	return -1
}

// IndexAll returns all byte offsets where pattern occurs in data.
// Non-overlapping matches only.
func IndexAll(data, pattern []byte) []int {
	plen := len(pattern)
	switch {
	case plen == 0:
		return nil
	case plen == 1:
		return indexAllByte(data, pattern[0])
	case plen > len(data):
		return nil
	}

	first := archsimd.BroadcastUint8x32(pattern[0])
	last := archsimd.BroadcastUint8x32(pattern[plen-1])

	// Start with stack-backed storage for small result sets (common case).
	// If more than 16 matches, append will allocate on heap.
	var stackBuf [16]int
	offsets := stackBuf[:0]
	i := 0
	limit := len(data) - plen + 1

	for i+32 <= limit {
		blockFirst := archsimd.LoadUint8x32Slice(data[i:])
		blockLast := archsimd.LoadUint8x32Slice(data[i+plen-1:])

		mFirst := blockFirst.Equal(first)
		mLast := blockLast.Equal(last)
		b := mFirst.And(mLast).ToBits()

		for b != 0 {
			j := bits.TrailingZeros32(b)
			pos := i + j
			candidate := data[pos : pos+plen]
			if plen <= 2 || bytes.Equal(candidate[1:plen-1], pattern[1:plen-1]) {
				offsets = append(offsets, pos)
				// Skip past this match to avoid overlapping
				// Clear all bits up to and including j+plen-1 to avoid overlaps
				skipTo := j + plen
				if skipTo < 32 {
					b >>= skipTo
					b <<= skipTo
				} else {
					b = 0
				}
				continue
			}
			b &= b - 1
		}

		i += 32
	}

	// Scalar tail
	for i < limit {
		idx := bytes.Index(data[i:], pattern)
		if idx < 0 {
			break
		}
		offsets = append(offsets, i+idx)
		i += idx + plen
	}

	archsimd.ClearAVXUpperBits()
	if len(offsets) == 0 {
		return nil
	}
	return offsets
}

// indexAllByte returns all byte offsets where byte c occurs in data.
func indexAllByte(data []byte, c byte) []int {
	var stackBuf [16]int
	offsets := stackBuf[:0]
	needle := archsimd.BroadcastUint8x32(c)
	i := 0

	for i+32 <= len(data) {
		chunk := archsimd.LoadUint8x32Slice(data[i:])
		mask := chunk.Equal(needle)
		b := mask.ToBits()
		for b != 0 {
			j := bits.TrailingZeros32(b)
			offsets = append(offsets, i+j)
			b &= b - 1
		}
		i += 32
	}

	for ; i < len(data); i++ {
		if data[i] == c {
			offsets = append(offsets, i)
		}
	}

	archsimd.ClearAVXUpperBits()
	if len(offsets) == 0 {
		return nil
	}
	return offsets
}

// IndexCaseInsensitive returns the index of the first case-insensitive occurrence of pattern in data.
// Pattern must be pre-lowered. Only handles ASCII case folding.
func IndexCaseInsensitive(data, patternLower []byte) int {
	plen := len(patternLower)
	switch {
	case plen == 0:
		return 0
	case plen > len(data):
		return -1
	}

	// For case-insensitive, we need to check both cases of first/last byte
	firstLo := patternLower[0]
	firstHi := toUpperASCII(firstLo)
	lastLo := patternLower[plen-1]
	lastHi := toUpperASCII(lastLo)

	bFirstLo := archsimd.BroadcastUint8x32(firstLo)
	bFirstHi := archsimd.BroadcastUint8x32(firstHi)
	bLastLo := archsimd.BroadcastUint8x32(lastLo)
	bLastHi := archsimd.BroadcastUint8x32(lastHi)

	i := 0
	limit := len(data) - plen + 1

	for i+32 <= limit {
		blockFirst := archsimd.LoadUint8x32Slice(data[i:])
		blockLast := archsimd.LoadUint8x32Slice(data[i+plen-1:])

		mFirstLo := blockFirst.Equal(bFirstLo)
		mFirstHi := blockFirst.Equal(bFirstHi)
		mFirst := mFirstLo.Or(mFirstHi)

		mLastLo := blockLast.Equal(bLastLo)
		mLastHi := blockLast.Equal(bLastHi)
		mLast := mLastLo.Or(mLastHi)

		b := mFirst.And(mLast).ToBits()

		for b != 0 {
			j := bits.TrailingZeros32(b)
			if matchCaseInsensitive(data[i+j:i+j+plen], patternLower) {
				archsimd.ClearAVXUpperBits()
				return i + j
			}
			b &= b - 1
		}

		i += 32
	}

	// Scalar tail
	for ; i < limit; i++ {
		if matchCaseInsensitive(data[i:i+plen], patternLower) {
			archsimd.ClearAVXUpperBits()
			return i
		}
	}

	archsimd.ClearAVXUpperBits()
	return -1
}

// IndexAllCaseInsensitive returns all byte offsets of case-insensitive, non-overlapping matches.
func IndexAllCaseInsensitive(data, patternLower []byte) []int {
	plen := len(patternLower)
	if plen == 0 || plen > len(data) {
		return nil
	}

	firstLo := patternLower[0]
	firstHi := toUpperASCII(firstLo)
	lastLo := patternLower[plen-1]
	lastHi := toUpperASCII(lastLo)

	bFirstLo := archsimd.BroadcastUint8x32(firstLo)
	bFirstHi := archsimd.BroadcastUint8x32(firstHi)
	bLastLo := archsimd.BroadcastUint8x32(lastLo)
	bLastHi := archsimd.BroadcastUint8x32(lastHi)

	var stackBuf [16]int
	offsets := stackBuf[:0]
	i := 0
	limit := len(data) - plen + 1

	for i+32 <= limit {
		blockFirst := archsimd.LoadUint8x32Slice(data[i:])
		blockLast := archsimd.LoadUint8x32Slice(data[i+plen-1:])

		mFirst := blockFirst.Equal(bFirstLo).Or(blockFirst.Equal(bFirstHi))
		mLast := blockLast.Equal(bLastLo).Or(blockLast.Equal(bLastHi))
		b := mFirst.And(mLast).ToBits()

		for b != 0 {
			j := bits.TrailingZeros32(b)
			pos := i + j
			if matchCaseInsensitive(data[pos:pos+plen], patternLower) {
				offsets = append(offsets, pos)
				skipTo := j + plen
				if skipTo < 32 {
					b >>= skipTo
					b <<= skipTo
				} else {
					b = 0
				}
				continue
			}
			b &= b - 1
		}

		i += 32
	}

	for ; i < limit; i++ {
		if matchCaseInsensitive(data[i:i+plen], patternLower) {
			offsets = append(offsets, i)
			i += plen - 1
		}
	}

	archsimd.ClearAVXUpperBits()
	if len(offsets) == 0 {
		return nil
	}
	return offsets
}

func matchCaseInsensitive(data, patternLower []byte) bool {
	for i, b := range data {
		if toLowerASCII(b) != patternLower[i] {
			return false
		}
	}
	return true
}

func toLowerASCII(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

func toUpperASCII(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - ('a' - 'A')
	}
	return b
}
