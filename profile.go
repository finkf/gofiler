package gofiler

// Profile maps unkown OCR token in a profiled document to the
// according interpreations of the profiler.
type Profile map[string]Interpretation

// Interpretation holds different candiates for unkown OCR tokens.
type Interpretation struct {
	OCR        string
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

// Pattern represents error patterns in strings.  Left represents the
// true pattern, Right the actuall pattern in the string at position
// Pos.
type Pattern struct {
	Left  string
	Right string
	Pos   int
}
