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
		if err := json.NewDecoder(r).Decode(&profile); err != nil {
			return fmt.Errorf("cannot decode profile: %v", err)
		}
		return nil
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
				return fmt.Errorf("read candidate: %v", err)
			}
			if err := f(ocr, cand); err != nil {
				return fmt.Errorf("read candidate: %v", err)
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
	// g, gctx := errgroup.WithContext(ctx)
	// stdin, pw := io.Pipe()
	// pr, stdout := io.Pipe()
	cmd := exec.CommandContext(ctx, p.Exe, args...)
	if p.Log != nil {
		p.Log.Log(fmt.Sprintf("cmd: %s %s", p.Exe, strings.Join(args, " ")))
		cmd.Stderr = &logwriter{logger: p.Log}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("run profiler: connect stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("run profiler: connect stdout: %v", err)
	}
	// Run profiler: write input and read output.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("run profiler: %v", err)
	}
	//if err := writeTokens(newWriterContext(ctx, stdin), tokens); err != nil {
	if err := writeTokens(stdin, tokens); err != nil {
		return fmt.Errorf("run profiler: %v", err)
	}
	// No need to close stdout; cmd takes care of this.
	//if err := f(newReaderContext(ctx, stdout)); err != nil {
	if err := f(stdout); err != nil {
		return fmt.Errorf("run profiler: %v", err)
	}
	// Wait for the command to finish.
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("run profiler: %v", err)
	}
	return nil
}

func writeTokens(w io.WriteCloser, ts []Token) error {
	defer w.Close()
	for _, t := range ts {
		if _, err := fmt.Fprintf(w, "%s\n", t); err != nil {
			return fmt.Errorf("write token %s: %v", t, err)
		}
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
