package gofiler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	var profile Profile
	err := p.run(ctx, config, tokens, args, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&profile)
	})
	return profile, err
}

// RunFunc profiles a list of tokens. It uses the given language
// configuration.  The optional logger is used to write the process's
// stderr.  The callback function is called for every Profiler
// suggestion.
func (p *Profiler) RunFunc(ctx context.Context, config string, tokens []Token, f func(string, Candidate) error) error {
	args := []string{
		"--config",
		config,
		"--sourceFormat",
		"EXT",
		"--sourceFile",
		"/dev/stdin",
		"--simpleOutput",
	}
	return p.run(ctx, config, tokens, args, func(r io.Reader) error {
		s := bufio.NewScanner(r)
		for s.Scan() {
			cand, ocr, err := MakeCandidate(s.Text())
			if err != nil {
				return err
			}
			if err := f(ocr, cand); err != nil {
				return err
			}
		}
		return s.Err()
	})
}

func (p *Profiler) run(ctx context.Context, config string, tokens []Token, args []string, f func(io.Reader) error) error {
	if p.Types {
		args = append(args, "--types")
	}
	if p.Adaptive {
		args = append(args, "--adaptive")
	}
	w, err := writeTokens(tokens)
	if err != nil {
		return fmt.Errorf("cannot write tokens: %v", err)
	}

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, p.Exe, args...)
	cmd.Stdin = w
	cmd.Stdout = &buf
	if p.Log != nil {
		p.Log.Log(fmt.Sprintf("cmd: %s", strings.Join(append([]string{p.Exe}, args...), " ")))
		cmd.Stderr = &logwriter{logger: p.Log}
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot profile tokens: %v", err)
	}
	if err := f(&buf); err != nil {
		return fmt.Errorf("cannot read profile: %v", err)
	}
	return nil
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
	var b bytes.Buffer
	for i := range tokens {
		if _, err := b.WriteString(tokens[i].String()); err != nil {
			return nil, err
		}
		if err := b.WriteByte('\n'); err != nil {
			return nil, err
		}
	}
	return &b, nil
}
