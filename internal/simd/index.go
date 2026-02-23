package simd

import (
	"bytes"
	"math/bits"

	"simd/archsimd"
)

// Index returns the index of the first occurrence of pattern in data, or -1 if not present.
// Delegates to bytes.Index which uses optimized AVX2 assembly internally.
func Index(data, pattern []byte) int {
	return bytes.Index(data, pattern)
}

// IndexAll returns all byte offsets where pattern occurs in data.
// Non-overlapping matches only. Uses bytes.Index (AVX2 asm) for the scan loop.
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

	// Collect into a non-escaping stack buffer first, then copy to heap
	// only if we found matches. This avoids a 128-byte heap alloc on no-match.
	var stackBuf [16]int
	n := 0
	var overflow []int
	i := 0

	for {
		idx := bytes.Index(data[i:], pattern)
		if idx < 0 {
			break
		}
		if n < len(stackBuf) {
			stackBuf[n] = i + idx
		} else {
			if overflow == nil {
				overflow = make([]int, 0, 64)
				overflow = append(overflow, stackBuf[:]...)
			}
			overflow = append(overflow, i+idx)
		}
		n++
		i += idx + plen
	}

	if n == 0 {
		return nil
	}
	if overflow != nil {
		return overflow
	}
	result := make([]int, n)
	copy(result, stackBuf[:n])
	return result
}

// indexAllByte returns all byte offsets where byte c occurs in data.
func indexAllByte(data []byte, c byte) []int {
	var stackBuf [16]int
	n := 0
	var overflow []int
	needle := archsimd.BroadcastUint8x32(c)
	i := 0

	for i+32 <= len(data) {
		chunk := archsimd.LoadUint8x32Slice(data[i:])
		mask := chunk.Equal(needle)
		b := mask.ToBits()
		for b != 0 {
			j := bits.TrailingZeros32(b)
			if n < len(stackBuf) {
				stackBuf[n] = i + j
			} else {
				if overflow == nil {
					overflow = make([]int, 0, 64)
					overflow = append(overflow, stackBuf[:]...)
				}
				overflow = append(overflow, i+j)
			}
			n++
			b &= b - 1
		}
		i += 32
	}

	for ; i < len(data); i++ {
		if data[i] == c {
			if n < len(stackBuf) {
				stackBuf[n] = i
			} else {
				if overflow == nil {
					overflow = make([]int, 0, 64)
					overflow = append(overflow, stackBuf[:]...)
				}
				overflow = append(overflow, i)
			}
			n++
		}
	}

	archsimd.ClearAVXUpperBits()
	if n == 0 {
		return nil
	}
	if overflow != nil {
		return overflow
	}
	result := make([]int, n)
	copy(result, stackBuf[:n])
	return result
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
	n := 0
	var overflow []int
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
				if n < len(stackBuf) {
					stackBuf[n] = pos
				} else {
					if overflow == nil {
						overflow = make([]int, 0, 64)
						overflow = append(overflow, stackBuf[:]...)
					}
					overflow = append(overflow, pos)
				}
				n++
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
			if n < len(stackBuf) {
				stackBuf[n] = i
			} else {
				if overflow == nil {
					overflow = make([]int, 0, 64)
					overflow = append(overflow, stackBuf[:]...)
				}
				overflow = append(overflow, i)
			}
			n++
			i += plen - 1
		}
	}

	archsimd.ClearAVXUpperBits()
	if n == 0 {
		return nil
	}
	if overflow != nil {
		return overflow
	}
	result := make([]int, n)
	copy(result, stackBuf[:n])
	return result
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
