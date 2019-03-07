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
// name and the according config paht in the backend directory.
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
// start with `#` all other tokens contain exactly on `/` to seperate
// the ocr token from the correction token. Tokens with no correction
// still end with `/` (they contain an empty correction string.
func (t Token) String() string {
	if len(t.LE) > 0 {
		return fmt.Sprintf("#%s", t.LE)
	}
	if len(t.COR) > 0 {
		return fmt.Sprintf("%s/%s", t.OCR, t.COR)
	}
	return t.OCR
}

// Logger defines a simple interface for the stderr logger of the
// profiling.
type Logger interface {
	Log(string)
}

// Run profiles a list of tokens. It uses the given executable with
// the given language configuration. The optional logger is used to
// write the process's stderr.
func Run(ctx context.Context, exe, config string, tokens []Token, l Logger) (Profile, error) {
	args := []string{
		"--config",
		config,
		"--types",
		"--sourceFormat",
		//		"TOKENS",
		"TXT",
		"--sourceFile",
		"/dev/stdin",
		"--jsonOutput",
		"/dev/stdout",
	}
	w, err := writeTokens(tokens)
	if err != nil {
		return nil, fmt.Errorf("cannot write tokens: %v", err)
	}
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdin = w
	cmd.Stdout = &stdout
	if l != nil {
		cmd.Stderr = &logwriter{logger: l}
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
		if len(tokens[i].LE) > 0 {
			w.writeByte('#')
			w.writeString(tokens[i].LE)
			w.writeByte('\n')
			continue
		}
		if len(tokens[i].COR) > 0 {
			w.writeString(tokens[i].OCR)
			w.writeByte('/')
			w.writeString(tokens[i].COR)
			w.writeByte('\n')
			continue
		}
		w.writeString(tokens[i].OCR)
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
