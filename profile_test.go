package gofiler

import (
	"encoding/json"
	"io"
	"os"
	"testing"
)

func withOpenProfile(f func(io.Reader)) {
	in, err := os.Open("testdata/profile.json")
	if err != nil {
		panic(err)
	}
	defer in.Close()
	f(in)
}

func TestReadProfileFromJSON(t *testing.T) {
	tests := []struct {
		ocr    string
		ncands int
	}{
		{"Vnheilfolles", 41},
		{"Waſſer", 6},
		{"empty", 0},
		{"null", 0},
	}

	for _, tc := range tests {
		t.Run(tc.ocr, func(t *testing.T) {
			withOpenProfile(func(in io.Reader) {
				profile := make(Profile)
				if err := json.NewDecoder(in).Decode(&profile); err != nil {
					t.Fatalf("got error: %v", err)
				}
				interpretation, ok := profile[tc.ocr]
				if !ok {
					t.Fatalf("cannot find %q in profile", tc.ocr)
				}
				if interpretation.OCR != tc.ocr {
					t.Fatalf("expected OCR=%q; got %q", tc.ocr, interpretation.OCR)
				}
				if got := len(interpretation.Candidates); got != tc.ncands {
					t.Fatalf("expected %d; got %d", tc.ncands, got)
				}
			})
		})
	}
}

func TestPatternString(t *testing.T) {
	for _, tc := range []struct {
		p    Pattern
		want string
	}{
		{Pattern{"a", "b", 0.0, 1}, "(a:b,1)"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.p.String(); got != tc.want {
				t.Fatalf("expected %s; got %s", tc.want, got)
			}
		})
	}
}

func TestCandidateString(t *testing.T) {
	for _, tc := range []struct {
		c    Candidate
		want string
	}{
		{
			Candidate{"sug", "modern", "dict",
				[]Pattern{{"a", "b", 0.0, 1}},
				[]Pattern{{"c", "d", 0.0, 3}}, 2, 1e-4},
			"sug:{modern+[(a:b,1)]}+ocr[(c:d,3)],voteWeight=1.000000e-04,levDistance=2,dict=dict",
		},
	} {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.c.String(); got != tc.want {
				t.Fatalf("expected %s; got %s", tc.want, got)
			}
		})
	}
}

func TestGlobalPatterns(t *testing.T) {
	tests := []struct {
		pat  string
		want float64
		ocr  bool
	}{
		{"u:v", 0.1, true},
		{"v:f", 0.2, true},
		{"un:vn", 0.3, false},
		{"u:û", 0.4, false},
	}
	for _, tc := range tests {
		t.Run(tc.pat, func(t *testing.T) {
			withOpenProfile(func(in io.Reader) {
				profile := make(Profile)
				if err := json.NewDecoder(in).Decode(&profile); err != nil {
					t.Fatalf("got error: %v", err)
				}
				got := profile.GlobalHistPatterns()[tc.pat]
				if tc.ocr {
					got = profile.GlobalOCRPatterns()[tc.pat]
				}
				if got != tc.want {
					t.Fatalf("expected %f; got %f", got, tc.want)
				}
			})
		})
	}
}
