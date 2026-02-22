package output

// Formatter formats a Result into bytes for output.
// buf is a reusable buffer — implementations append to it and return the result.
// Callers can pass buf[:0] to reuse the underlying array without allocating.
type Formatter interface {
	Format(buf []byte, result Result, multiFile bool) []byte
}
