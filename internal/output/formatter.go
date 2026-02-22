package output

// Formatter formats a Result into bytes for output.
type Formatter interface {
	Format(result Result, multiFile bool) []byte
}
