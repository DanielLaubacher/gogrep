package matcher

import (
	"fmt"
	"strings"
)

// MatcherOpts holds display-related options that affect match extraction.
type MatcherOpts struct {
	MaxCols      int  // max columns for snippet extraction (0 = full lines)
	NeedLineNums bool // compute line numbers (false = skip for speed)
}

// NewMatcher creates the appropriate Matcher based on the provided options.
// Selection logic:
//   - PCRE flag -> PCREMatcher (PCRE2 via pure Go port)
//   - Fixed + 1 pattern -> BoyerMooreMatcher (sublinear search)
//   - Fixed + N patterns -> AhoCorasickMatcher (single-pass multi-pattern)
//   - Otherwise -> RegexMatcher (RE2)
func NewMatcher(patterns []string, fixed bool, usePCRE bool, ignoreCase bool, invert bool, opts MatcherOpts) (Matcher, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no patterns provided")
	}

	if usePCRE {
		// Combine multiple patterns with |
		pattern := patterns[0]
		if len(patterns) > 1 {
			combined := ""
			for i, p := range patterns {
				if i > 0 {
					combined += "|"
				}
				combined += "(?:" + p + ")"
			}
			pattern = combined
		}
		m, err := NewPCREMatcher(pattern, ignoreCase, invert)
		if err != nil {
			return nil, err
		}
		m.maxCols = opts.MaxCols
		m.needLineNums = opts.NeedLineNums
		return m, nil
	}

	if fixed {
		if len(patterns) == 1 {
			m := NewBoyerMooreMatcher(patterns[0], ignoreCase, invert)
			m.maxCols = opts.MaxCols
			m.needLineNums = opts.NeedLineNums
			return m, nil
		}
		m := NewAhoCorasickMatcher(patterns, ignoreCase, invert)
		m.maxCols = opts.MaxCols
		m.needLineNums = opts.NeedLineNums
		return m, nil
	}

	// Optimization: if all patterns are literal strings (no regex metacharacters),
	// use BoyerMooreMatcher / AhoCorasickMatcher for SIMD-accelerated search.
	allLiteral := true
	for _, p := range patterns {
		if !isLiteral(p) {
			allLiteral = false
			break
		}
	}
	if allLiteral {
		if len(patterns) == 1 {
			m := NewBoyerMooreMatcher(patterns[0], ignoreCase, invert)
			m.maxCols = opts.MaxCols
			m.needLineNums = opts.NeedLineNums
			return m, nil
		}
		m := NewAhoCorasickMatcher(patterns, ignoreCase, invert)
		m.maxCols = opts.MaxCols
		m.needLineNums = opts.NeedLineNums
		return m, nil
	}

	// Regex mode: combine multiple patterns with |
	pattern := patterns[0]
	if len(patterns) > 1 {
		combined := ""
		for i, p := range patterns {
			if i > 0 {
				combined += "|"
			}
			combined += "(?:" + p + ")"
		}
		pattern = combined
	}

	m, err := NewRegexMatcher(pattern, ignoreCase, invert)
	if err != nil {
		return nil, err
	}
	m.maxCols = opts.MaxCols
	m.needLineNums = opts.NeedLineNums
	return m, nil
}

// isLiteral returns true if the pattern contains no regex metacharacters
// and can be treated as a fixed string.
func isLiteral(pattern string) bool {
	return !strings.ContainsAny(pattern, `\.+*?()|[]{}^$`)
}
