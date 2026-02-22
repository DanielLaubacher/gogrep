package matcher

import "bytes"

// acNode is a node in the Aho-Corasick automaton.
type acNode struct {
	children [256]*acNode
	fail     *acNode
	output   []int // indices of patterns that match at this node
	depth    int
}

// AhoCorasickMatcher matches multiple fixed patterns simultaneously
// using the Aho-Corasick algorithm.
type AhoCorasickMatcher struct {
	root       *acNode
	patterns   [][]byte // original patterns
	ignoreCase bool
	invert     bool
}

// NewAhoCorasickMatcher creates an AhoCorasickMatcher for multiple fixed patterns.
func NewAhoCorasickMatcher(patterns []string, ignoreCase bool, invert bool) *AhoCorasickMatcher {
	m := &AhoCorasickMatcher{
		root:       &acNode{},
		ignoreCase: ignoreCase,
		invert:     invert,
	}

	// Build the trie
	for i, p := range patterns {
		pat := []byte(p)
		if ignoreCase {
			pat = bytes.ToLower(pat)
		}
		m.patterns = append(m.patterns, pat)
		m.addPattern(pat, i)
	}

	// Build failure links via BFS
	m.buildFailureLinks()

	return m
}

func (m *AhoCorasickMatcher) addPattern(pattern []byte, index int) {
	node := m.root
	for _, b := range pattern {
		if node.children[b] == nil {
			node.children[b] = &acNode{depth: node.depth + 1}
		}
		node = node.children[b]
	}
	node.output = append(node.output, index)
}

func (m *AhoCorasickMatcher) buildFailureLinks() {
	// BFS from root
	queue := make([]*acNode, 0, 256)

	// Initialize depth-1 nodes: fail links point to root
	for i := range 256 {
		child := m.root.children[i]
		if child != nil {
			child.fail = m.root
			queue = append(queue, child)
		}
	}

	// BFS to build failure links
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for i := range 256 {
			child := current.children[i]
			if child == nil {
				continue
			}

			queue = append(queue, child)

			// Follow failure links to find the longest proper suffix
			fail := current.fail
			for fail != nil && fail.children[i] == nil {
				fail = fail.fail
			}
			if fail == nil {
				child.fail = m.root
			} else {
				child.fail = fail.children[i]
			}

			// Merge output from failure node
			if child.fail != nil && len(child.fail.output) > 0 {
				child.output = append(child.output, child.fail.output...)
			}
		}
	}
}

// acMatch represents a single pattern match at a byte offset.
type acMatch struct {
	patternIdx int
	offset     int // byte offset in the searched text
	length     int // length of the matched pattern
}

// searchLine scans a single line for all pattern matches.
func (m *AhoCorasickMatcher) searchLine(text []byte) []acMatch {
	var matches []acMatch
	node := m.root

	for i, b := range text {
		if m.ignoreCase {
			b = toLower(b)
		}

		// Follow failure links until we find a matching transition or reach root
		for node != m.root && node.children[b] == nil {
			node = node.fail
		}
		if node.children[b] != nil {
			node = node.children[b]
		}

		// Collect all matches at this position
		if len(node.output) > 0 {
			for _, pidx := range node.output {
				plen := len(m.patterns[pidx])
				matches = append(matches, acMatch{
					patternIdx: pidx,
					offset:     i - plen + 1,
					length:     plen,
				})
			}
		}
	}

	return matches
}

func (m *AhoCorasickMatcher) MatchExists(data []byte) bool {
	if m.invert {
		return len(data) > 0
	}
	// Walk automaton until first match
	node := m.root
	for _, b := range data {
		if m.ignoreCase {
			b = toLower(b)
		}
		for node != m.root && node.children[b] == nil {
			node = node.fail
		}
		if node.children[b] != nil {
			node = node.children[b]
		}
		if len(node.output) > 0 {
			return true
		}
	}
	return false
}

func (m *AhoCorasickMatcher) FindAll(data []byte) []Match {
	if m.invert {
		return m.findAllInvert(data)
	}

	// Search whole buffer with automaton
	acMatches := m.searchLine(data)
	if len(acMatches) == 0 {
		return nil
	}

	cursor := newLineCursor(data)
	matches := make([]Match, 0, len(acMatches))
	allPos := make([][2]int, 0, len(acMatches))
	lastLineStart := int64(-1)
	posStart := 0

	for _, am := range acMatches {
		line, lineOffset, lineNum := cursor.lineFromPos(am.offset)

		posInLine := am.offset - int(lineOffset)
		posEnd := posInLine + am.length

		allPos = append(allPos, [2]int{posInLine, posEnd})

		if lineOffset == lastLineStart {
			last := &matches[len(matches)-1]
			last.Positions = allPos[posStart:]
		} else {
			posStart = len(allPos) - 1
			matches = append(matches, Match{
				LineNum:    lineNum,
				LineBytes:  line,
				ByteOffset: lineOffset,
				Positions:  allPos[posStart:],
			})
			lastLineStart = lineOffset
		}
	}

	return matches
}

func (m *AhoCorasickMatcher) findAllInvert(data []byte) []Match {
	var matches []Match
	var offset int64
	lineNum := 1

	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		var line []byte
		if idx >= 0 {
			line = data[:idx]
			data = data[idx+1:]
		} else {
			line = data
			data = nil
		}

		if len(m.searchLine(line)) == 0 {
			matches = append(matches, Match{
				LineNum:    lineNum,
				LineBytes:  line,
				ByteOffset: offset,
			})
		}

		offset += int64(len(line)) + 1
		lineNum++
	}

	return matches
}

func (m *AhoCorasickMatcher) FindLine(line []byte, lineNum int, byteOffset int64) (Match, bool) {
	return m.findInLine(line, lineNum, byteOffset)
}

func (m *AhoCorasickMatcher) findInLine(line []byte, lineNum int, byteOffset int64) (Match, bool) {
	acMatches := m.searchLine(line)
	hasMatch := len(acMatches) > 0

	if m.invert {
		hasMatch = !hasMatch
	}

	if !hasMatch {
		return Match{}, false
	}

	match := Match{
		LineNum:    lineNum,
		LineBytes:  line,
		ByteOffset: byteOffset,
	}
	if !m.invert {
		match.Positions = make([][2]int, len(acMatches))
		for i, am := range acMatches {
			match.Positions[i] = [2]int{am.offset, am.offset + am.length}
		}
	}

	return match, true
}
