package input

import (
	"io"
	"os"
)

// StdinReader reads all data from stdin.
type StdinReader struct{}

// NewStdinReader creates a new StdinReader.
func NewStdinReader() *StdinReader {
	return &StdinReader{}
}

func (r *StdinReader) Read(_ string) (ReadResult, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return ReadResult{}, err
	}
	return ReadResult{
		Data:   data,
		Closer: func() error { return nil },
	}, nil
}
