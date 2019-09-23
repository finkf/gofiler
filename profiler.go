package gofiler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrorLanguageNotFound is the error that is returned if a language
// configuration cannot be found.
var ErrorLanguageNotFound = errors.New("laguage configuration not found")

// FindLanguage searches the backend directory for a language
// configuration. It returns ErrorLanguageNotFound if the language
// configuration cannot be found.
func FindLanguage(backend, language string) (LanguageConfiguration, error) {
	lcs, err := ListLanguages(backend)
	if err != nil {
		return LanguageConfiguration{}, err
	}
	search := strings.ToLower(language)
	for _, lc := range lcs {
		if strings.ToLower(lc.Language) == search {
			return lc, nil
		}
	}
	return LanguageConfiguration{}, ErrorLanguageNotFound
}

// LanguageConfiguration represents a pair that consists of a language
// name and the according config path in the backend directory.
type LanguageConfiguration struct {
	Language, Path string
}

// ListLanguages returns a list of language configurations in the
// given backend directory.
func ListLanguages(backend string) ([]LanguageConfiguration, error) {
	suf := ".ini"
	fis, err := ioutil.ReadDir(backend)
	if err != nil {
		return nil, fmt.Errorf("cannot list languages: %v", err)
	}
	var lcs []LanguageConfiguration
	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}
		name := fi.Name()
		if strings.HasSuffix(name, suf) {
			lcs = append(lcs, LanguageConfiguration{
				Language: strings.ToLower(name[0 : len(name)-len(suf)]),
				Path:     filepath.Join(backend, name),
			})
		}
	}
	return lcs, nil
}

// Token represents an input token for the profiling.  A token either
// contains an entry for the extended lexicon (LE) or a text token
// (OCR) with an optional manual correction (COR).
//
// Tokens must never contain any whitespace in any of the strings.
type Token struct {
	LE, OCR, COR string
}

// String implements the io.Stringer interface.  The output is
// suitable as direct input for the profiler, i.e each lexicon entry
// start with `#` all other tokens contain exactly on `:` to seperate
// the ocr token from the correction token. Tokens with no correction
// still must end with `:` (they contain an empty correction string).
func (t Token) String() string {
	if len(t.LE) > 0 {
		return fmt.Sprintf("#%s", t.LE)
	}
	return fmt.Sprintf("%s:%s", t.OCR, t.COR)
}

// Logger defines a simple interface for the stderr logger of the
// profiling.
type Logger interface {
	Log(string)
}

// Profiler is a profiler executable with an optional logger and some
// minor options.
type Profiler struct {
	Exe             string
	Log             Logger
	Types, Adaptive bool
}

// Run profiles a list of tokens. It uses the given executable with
// the given language configuration. The optional logger is used to
// write the process's stderr.
func (p *Profiler) Run(ctx context.Context, config string, tokens []Token) (Profile, error) {
	args := []string{
		"--config",
		config,
		"--sourceFormat",
		"EXT",
		"--sourceFile",
		"/dev/stdin",
		"--jsonOutput",
		"/dev/stdout",
	}
	if p.Types {
		args = append(args, "--types")
	}
	if p.Adaptive {
		args = append(args, "--adaptive")
	}
	w, err := writeTokens(tokens)
	if err != nil {
		return nil, fmt.Errorf("cannot write tokens: %v", err)
	}

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, p.Exe, args...)
	cmd.Stdin = w
	cmd.Stdout = &stdout
	if p.Log != nil {
		cmd.Stderr = &logwriter{logger: p.Log}
	}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cannot profile tokens: %v", err)
	}
	var profile Profile
	if err := json.NewDecoder(&stdout).Decode(&profile); err != nil {
		return nil, fmt.Errorf("cannot read profile: %v", err)
	}
	return profile, nil
}

type logwriter struct {
	logger Logger
	buffer []byte
}

func (l *logwriter) Write(p []byte) (int, error) {
	l.buffer = append(l.buffer, p...)
	for pos := bytes.IndexByte(l.buffer, '\n'); pos != -1; pos = bytes.IndexByte(l.buffer, '\n') {
		l.logger.Log(string(l.buffer[:pos]))
		l.buffer = l.buffer[pos+1:]
	}
	return len(p), nil
}

func writeTokens(tokens []Token) (*bytes.Buffer, error) {
	w := tokenwriter{b: &bytes.Buffer{}}
	for i := 0; i < len(tokens) && w.err == nil; i++ {
		w.writeString(tokens[i].String())
		w.writeByte('\n')
	}
	return w.b, w.err
}

type tokenwriter struct {
	err error
	b   *bytes.Buffer
}

func (w *tokenwriter) writeByte(b byte) {
	if w.err != nil {
		return
	}
	if err := w.b.WriteByte(b); err != nil {
		w.err = err
	}
}

func (w *tokenwriter) writeString(str string) {
	if w.err != nil {
		return
	}
	if _, err := w.b.WriteString(str); err != nil {
		w.err = err
	}
}
