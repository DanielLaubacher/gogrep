package walker

import "bytes"

// IsBinary checks if data appears to be binary by scanning for NUL bytes
// in the first 8KB, matching GNU grep behavior.
func IsBinary(data []byte) bool {
	limit := 8192
	if len(data) < limit {
		limit = len(data)
	}
	return bytes.IndexByte(data[:limit], 0) >= 0
}
