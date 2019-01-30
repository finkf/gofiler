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
