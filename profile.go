package gofiler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Profile maps unkown OCR token in a profiled document to the
// according interpreations of the profiler.
type Profile map[string]Interpretation

// GlobalHistPatterns returns all global historical patterns with
// their according probabilities.
func (p Profile) GlobalHistPatterns() map[string]float64 {
	ret := make(map[string]float64)
	for _, i := range p {
		for _, c := range i.Candidates {
			for _, p := range c.HistPatterns {
				ret[p.Left+":"+p.Right] = p.Prob
			}
		}
	}
	return ret
}

// GlobalOCRPatterns returns all global ocr error patterns with their
// according probabilities.
func (p Profile) GlobalOCRPatterns() map[string]float64 {
	ret := make(map[string]float64)
	for _, i := range p {
		for _, c := range i.Candidates {
			for _, p := range c.OCRPatterns {
				ret[p.Left+":"+p.Right] = p.Prob
			}
		}
	}
	return ret
}

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

// theyl@theil:{teil+[(t:th,0)]}+ocr[(i:y,3)],voteWeight=0.749764,levDistance=1,dict=dict_modern_hypothetic_error
func MakeCandidate(expr string) (Candidate, string, error) {
	var re = regexp.MustCompile(`(.*)@(.*):\{(.*)\+\[(.*)\]\}\+ocr\[(.*)\],voteWeight=(.*),levDistance=(\d*),dict=(.*)`)
	fail := func(err error) (Candidate, string, error) {
		return Candidate{}, "", fmt.Errorf("make candidate: %v", err)
	}
	m := re.FindStringSubmatch(expr)
	if m == nil {
		return fail(fmt.Errorf("bad expression %s", expr))
	}
	dist, err := strconv.Atoi(m[7])
	if err != nil {
		return fail(fmt.Errorf("bad expression %s: %v", expr, err))
	}
	weight, err := strconv.ParseFloat(m[6], 32)
	if err != nil {
		return fail(fmt.Errorf("bad expression %s: %v", expr, err))
	}
	hpats, err := str2ps(m[4])
	if err != nil {
		return fail(fmt.Errorf("bad expression %s: %v", expr, err))
	}
	opats, err := str2ps(m[5])
	if err != nil {
		return fail(fmt.Errorf("bad expression %s:%v", expr, err))
	}
	return Candidate{
		Suggestion:   m[2],
		Modern:       m[3],
		Weight:       float32(weight),
		Distance:     dist,
		Dict:         m[8],
		HistPatterns: hpats,
		OCRPatterns:  opats,
	}, m[1], nil
}

func (c Candidate) String() string {
	return fmt.Sprintf(
		"%s:{%s+[%s]}+ocr[%s],voteWeight=%g,levDistance=%d,dict=%s",
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
	for _, p := range ps {
		b.WriteString(p.String())
	}
	return b.String()
}

func str2ps(expr string) ([]Pattern, error) {
	var re = regexp.MustCompile(`((\([^)]*\)))`)
	m := re.FindAllString(expr, -1)
	if m == nil {
		return nil, nil
	}
	var ret []Pattern
	for i := range m {
		p, err := MakePattern(m[i])
		if err != nil {
			return nil, fmt.Errorf("bad pattern %s: %v", expr, err)
		}
		ret = append(ret, p)
	}
	return ret, nil
}

// Pattern represents error patterns in strings.  Left represents the
// `true` pattern(either the error correction or the modern form) and
// Right the actual pattern in the string at position Pos.
type Pattern struct {
	Left  string  // Left part of the pattern
	Right string  // Right part of the pattern
	Prob  float64 // Global probability of the pattern
	Pos   int     // Position
}

// MakePattern creates a pattern from a pattern expression `(left:right,pos)`.
func MakePattern(expr string) (Pattern, error) {
	var re = regexp.MustCompile(`\((.*):(.*),(\d*)\)`)
	m := re.FindStringSubmatch(expr)
	if m == nil {
		return Pattern{}, fmt.Errorf("make pattern: bad expression: %s", expr)
	}
	pos, _ := strconv.Atoi(m[3])
	return Pattern{
		Left:  m[1],
		Right: m[2],
		Pos:   pos,
	}, nil
}

func (p Pattern) String() string {
	return fmt.Sprintf("(%s:%s,%d)", p.Left, p.Right, p.Pos)
}
