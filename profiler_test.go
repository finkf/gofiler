package gofiler

import (
	"context"
	"testing"
	"time"
)

func TestListLanguages(t *testing.T) {
	lcs, err := ListLanguages("testdata")
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if got := len(lcs); got != 4 {
		t.Fatalf("expected %d language configurations; got %d", 4, got)
	}
}

func TestFindLanguage(t *testing.T) {
	tests := []struct {
		language string
		want     interface{}
	}{
		{"german", LanguageConfiguration{"german", "testdata/german.ini"}},
		{"Latin", LanguageConfiguration{"latin", "testdata/latin.ini"}},
		{"LATIN", LanguageConfiguration{"latin", "testdata/latin.ini"}},
		{"English", LanguageConfiguration{"english", "testdata/english.ini"}},
		{"GREEK", LanguageConfiguration{"greek", "testdata/greek.ini"}},
		{"no-such-language", ErrorLanguageNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.language, func(t *testing.T) {
			lc, err := FindLanguage("testdata", tc.language)
			if !(err == tc.want || lc == tc.want) {
				t.Fatalf("exepected %v; got %v, %v", tc.want, lc, err)
			}
		})
	}
}

type testLogger struct {
	got  string
	want string
}

func newTestLogger() *testLogger {
	var l testLogger
	for _, token := range tokens {
		l.want += token.String() + ";"
	}
	return &l
}

func (l *testLogger) Log(str string) {
	l.got += str + ";"
}

var tokens = []Token{
	{LE: "LE entry 1"},
	{LE: "LE entry 2"},
	{OCR: "OCR1", COR: "COR1"},
	{OCR: "OCR2", COR: "COR2"},
	{OCR: "OCR3"},
	{OCR: "OCR4"},
}

func TestRunWithLogger(t *testing.T) {
	l := newTestLogger()
	_, err := Run(context.Background(), "testdata/run_profiler.bash", "", tokens, l)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if l.got != l.want {
		t.Fatalf("expected %q got %q", l.got, l.want)
	}
}

func TestRunNoLogger(t *testing.T) {
	_, err := Run(context.Background(), "testdata/run_profiler.bash", "", tokens, nil)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestRunTimeOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := Run(ctx, "testdata/run_profiler_block.bash", "", tokens, nil)
	if err == nil {
		t.Fatalf("expected an error")
	}
}
