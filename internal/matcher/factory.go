package matcher

import (
	"fmt"
	"strings"
)

// NewMatcher creates the appropriate Matcher based on the provided options.
// Selection logic:
//   - PCRE flag -> PCREMatcher (PCRE2 via pure Go port)
//   - Fixed + 1 pattern -> BoyerMooreMatcher (sublinear search)
//   - Fixed + N patterns -> AhoCorasickMatcher (single-pass multi-pattern)
//   - Otherwise -> RegexMatcher (RE2)
func NewMatcher(patterns []string, fixed bool, usePCRE bool, ignoreCase bool, invert bool) (Matcher, error) {
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
		return NewPCREMatcher(pattern, ignoreCase, invert)
	}

	if fixed {
		if len(patterns) == 1 {
			return NewBoyerMooreMatcher(patterns[0], ignoreCase, invert), nil
		}
		return NewAhoCorasickMatcher(patterns, ignoreCase, invert), nil
	}

	// Optimization: if all patterns are literal strings (no regex metacharacters),
	// use BoyerMooreMatcher / AhoCorasickMatcher for SIMD-accelerated search.
	// This is the same optimization ripgrep does — detect literals and bypass the regex engine.
	allLiteral := true
	for _, p := range patterns {
		if !isLiteral(p) {
			allLiteral = false
			break
		}
	}
	if allLiteral {
		if len(patterns) == 1 {
			return NewBoyerMooreMatcher(patterns[0], ignoreCase, invert), nil
		}
		return NewAhoCorasickMatcher(patterns, ignoreCase, invert), nil
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

	return NewRegexMatcher(pattern, ignoreCase, invert)
}

// isLiteral returns true if the pattern contains no regex metacharacters
// and can be treated as a fixed string.
func isLiteral(pattern string) bool {
	return !strings.ContainsAny(pattern, `\.+*?()|[]{}^$`)
}
