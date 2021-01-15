package gofiler

import (
	"fmt"
	"strings"
)

// Profile maps unkown OCR token in a profiled document to the
// according interpreations of the profiler.
type Profile map[string]Interpretation

// Interpretation holds the list of candiates for OCR tokens.  In the
// case of lexicon entries, an interpretation holds only one candidate
// with empty historical and and ocr pattern list.
type Interpretation struct {
	OCR        string
	N          int
	Candidates []Candidate
}

// Candidate represents a correction candidate for an OCR token.
type Candidate struct {
	Suggestion   string    // Correction suggestion
	Modern       string    // Modern variant
	Dict         string    // Name of the used dictionary
	HistPatterns []Pattern // List of historical patterns
	OCRPatterns  []Pattern // List of OCR error patterns
	Distance     int       // Levenshtein distance
	Weight       float32   // The vote weight of the candidate
}

func (c Candidate) String() string {
	return fmt.Sprintf(
		"%s:{%s+%s}+ocr%s,voteWeight=%e,levDistance=%d,dict=%s",
		c.Suggestion,
		c.Modern,
		ps2str(c.HistPatterns),
		ps2str(c.OCRPatterns),
		c.Weight,
		c.Distance,
		c.Dict,
	)
}

func ps2str(ps []Pattern) string {
	var b strings.Builder
	b.WriteByte('[')
	for _, p := range ps {
		b.WriteString(p.String())
	}
	b.WriteByte(']')
	return b.String()
}

// Pattern represents error patterns in strings.  Left represents the
// `true` pattern, Right the actuall pattern in the string at position
// Pos.
type Pattern struct {
	Left  string  // Left part of the pattern
	Right string  // Right part of the pattern
	Prob  float64 // Global probability of the pattern
	Pos   int     // Position
}

func (p Pattern) String() string {
	return fmt.Sprintf("(%s:%s,%d)", p.Left, p.Right, p.Pos)
}
