package output

import (
	"encoding/json"
)

// JSONFormatter formats results as JSON Lines (one JSON object per match).
type JSONFormatter struct{}

// NewJSONFormatter creates a JSONFormatter.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// jsonMatch is the JSON serialization format for a match line.
type jsonMatch struct {
	Type       string    `json:"type"`
	File       string    `json:"file,omitempty"`
	LineNum    int       `json:"line_number"`
	ByteOffset int64     `json:"byte_offset"`
	Text       string    `json:"text"`
	Matches    []jsonPos `json:"matches,omitempty"`
}

type jsonPos struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (f *JSONFormatter) Format(buf []byte, result Result, multiFile bool) []byte {
	if len(result.Matches) == 0 {
		return buf
	}

	for _, m := range result.Matches {
		if m.IsContext {
			continue
		}
		jm := jsonMatch{
			Type:       "match",
			File:       result.FilePath,
			LineNum:    m.LineNum,
			ByteOffset: m.ByteOffset,
			Text:       string(m.LineBytes),
		}
		if len(m.Positions) > 0 {
			jm.Matches = make([]jsonPos, len(m.Positions))
			for i, pos := range m.Positions {
				jm.Matches[i] = jsonPos{Start: pos[0], End: pos[1]}
			}
		}
		data, _ := json.Marshal(jm)
		buf = append(buf, data...)
		buf = append(buf, '\n')
	}
	return buf
}

// Ensure JSONFormatter implements Formatter.
var _ Formatter = (*JSONFormatter)(nil)
